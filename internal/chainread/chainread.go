// Package chainread reads public chain state — a native balance, an ERC-20 balance and
// its metadata, the chain id — straight from JSON-RPC nodes, with failover across a
// list of endpoints. It is the CLI's only direct-to-node path: everything that goes
// through the gateway uses rail0-go, while this exists for reads the gateway does not
// serve (an arbitrary address's balance), against the keyless endpoints the gateway
// publishes per chain (Blockchain.PublicRPCURLs). Read-only: it never signs or sends.
package chainread

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// DefaultTimeout bounds each attempt against one endpoint. Short on purpose: a public
// endpoint that hangs should hand over to the next one quickly, not eat the whole
// command's patience.
const DefaultTimeout = 8 * time.Second

// erc20ABI is the read-only slice of ERC-20 the reader needs.
const erc20ABI = `[
 {"name":"balanceOf","type":"function","stateMutability":"view","inputs":[{"name":"owner","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
 {"name":"decimals","type":"function","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"uint8"}]},
 {"name":"symbol","type":"function","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"string"}]}
]`

var parsedERC20 = func() abi.ABI {
	a, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		panic(err)
	}
	return a
}()

// Reader reads from a list of RPC endpoints in failover order.
type Reader struct {
	urls    []string
	timeout time.Duration

	mu      sync.Mutex
	clients map[string]*ethclient.Client
	start   int // index of the endpoint that last answered: later calls begin there
}

// New returns a Reader over urls (tried in order). timeout <= 0 means DefaultTimeout.
func New(urls []string, timeout time.Duration) (*Reader, error) {
	var clean []string
	for _, u := range urls {
		if u = strings.TrimSpace(u); u != "" {
			clean = append(clean, u)
		}
	}
	if len(clean) == 0 {
		return nil, errors.New("no RPC endpoint to read from")
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Reader{urls: clean, timeout: timeout, clients: map[string]*ethclient.Client{}}, nil
}

// Close releases the underlying clients.
func (r *Reader) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.clients {
		c.Close()
	}
	r.clients = map[string]*ethclient.Client{}
}

// do runs fn against the endpoints in turn, each under its own timeout, and returns the
// first answer.
//
// Failover is for TRANSPORT failures only: an endpoint that is down, times out, or
// answers with an HTTP error. A JSON-RPC error (rpc.Error — e.g. "execution reverted"
// for a token address that is not an ERC-20) is the chain's answer, identical on every
// node, so it is returned at once instead of being retried everywhere. The endpoint
// that answered becomes the first one tried by the next call, so a dead first URL costs
// one timeout per command, not one per read. Errors name the endpoint's host only — a
// URL may carry a key in its path or query, and this text ends up in logs.
func (r *Reader) do(ctx context.Context, fn func(context.Context, *ethclient.Client) error) error {
	r.mu.Lock()
	start := r.start
	r.mu.Unlock()
	var errs []error
	for i := range r.urls {
		idx := (start + i) % len(r.urls)
		u := r.urls[idx]
		c, err := r.client(ctx, u)
		if err == nil {
			attempt, cancel := context.WithTimeout(ctx, r.timeout)
			err = fn(attempt, c)
			cancel()
			if err == nil {
				r.mu.Lock()
				r.start = idx
				r.mu.Unlock()
				return nil
			}
			var rpcErr rpc.Error
			if errors.As(err, &rpcErr) {
				return err
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		errs = append(errs, fmt.Errorf("%s: %w", hostOf(u), err))
	}
	return fmt.Errorf("no RPC endpoint answered: %w", errors.Join(errs...))
}

func (r *Reader) client(ctx context.Context, u string) (*ethclient.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clients[u]; ok {
		return c, nil
	}
	c, err := ethclient.DialContext(ctx, u)
	if err != nil {
		return nil, err
	}
	r.clients[u] = c
	return c, nil
}

// hostOf trims an endpoint to scheme://host for error messages (see do).
func hostOf(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			rest = rest[:j]
		}
		if k := strings.LastIndex(rest, "@"); k >= 0 { // user:pass@host
			rest = rest[k+1:]
		}
		return u[:i+3] + rest
	}
	return u
}

// ChainID asks the node which chain it serves (eth_chainId).
func (r *Reader) ChainID(ctx context.Context) (*big.Int, error) {
	var out *big.Int
	err := r.do(ctx, func(ctx context.Context, c *ethclient.Client) (err error) {
		out, err = c.ChainID(ctx)
		return err
	})
	return out, err
}

// NativeBalance is the address's balance of the chain's native coin at the latest
// block (eth_getBalance), in wei.
func (r *Reader) NativeBalance(ctx context.Context, addr common.Address) (*big.Int, error) {
	var out *big.Int
	err := r.do(ctx, func(ctx context.Context, c *ethclient.Client) (err error) {
		out, err = c.BalanceAt(ctx, addr, nil)
		return err
	})
	return out, err
}

// TokenBalance is balanceOf(holder) on the ERC-20 at token, in base units.
func (r *Reader) TokenBalance(ctx context.Context, token, holder common.Address) (*big.Int, error) {
	var out *big.Int
	err := r.call(ctx, token, "balanceOf", func(v []any) error {
		b, ok := v[0].(*big.Int)
		if !ok {
			return errors.New("balanceOf: unexpected result type")
		}
		out = b
		return nil
	}, holder)
	return out, err
}

// TokenMeta reads symbol() and decimals() of the ERC-20 at token.
func (r *Reader) TokenMeta(ctx context.Context, token common.Address) (symbol string, decimals uint8, err error) {
	err = r.call(ctx, token, "decimals", func(v []any) error {
		d, ok := v[0].(uint8)
		if !ok {
			return errors.New("decimals: unexpected result type")
		}
		decimals = d
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	err = r.call(ctx, token, "symbol", func(v []any) error {
		s, ok := v[0].(string)
		if !ok {
			return errors.New("symbol: unexpected result type")
		}
		symbol = s
		return nil
	})
	return symbol, decimals, err
}

// call ABI-encodes method(args...) for an eth_call to contract at the latest block and
// hands the decoded outputs to decode. An empty answer (no code at the address) is an
// error rather than a zero: reading "0" from a typo'd address would look like a balance.
func (r *Reader) call(ctx context.Context, contract common.Address, method string, decode func([]any) error, args ...any) error {
	data, err := parsedERC20.Pack(method, args...)
	if err != nil {
		return fmt.Errorf("encode %s: %w", method, err)
	}
	var raw []byte
	err = r.do(ctx, func(ctx context.Context, c *ethclient.Client) (err error) {
		raw, err = c.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
		return err
	})
	if err != nil {
		return fmt.Errorf("%s on %s: %w", method, contract.Hex(), err)
	}
	if len(raw) == 0 {
		return fmt.Errorf("%s on %s: empty result — no ERC-20 contract at that address on this chain", method, contract.Hex())
	}
	out, err := parsedERC20.Unpack(method, raw)
	if err != nil || len(out) == 0 {
		return fmt.Errorf("decode %s on %s: not an ERC-20 answer (%v)", method, contract.Hex(), err)
	}
	return decode(out)
}
