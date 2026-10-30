package cmd

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rail0/cli/internal/chainread"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	keysBalanceChainID int
	keysBalanceRPCURL  string
	keysBalanceToken   string
	keysBalanceTokens  bool
)

// nativeDecimals is the scale of every EVM chain's native coin (wei). Arc included: its
// native gas token is USDC with 18 decimals, while the USDC ERC-20 there has 6.
const nativeDecimals = 18

var keysBalanceCmd = &cobra.Command{
	Use:   "balance <address|@name|name>",
	Short: "Read an address's on-chain balance straight from the chain (like cast balance)",
	Long: `Read the native and/or ERC-20 balance of an address directly from the chain's
RPC nodes — no gateway session, nothing signed.

The address is a 0x address, or the name of a saved key ('rail0 keys add'), with or
without the @: its address is derived from the keychain entry, the key itself is
never printed. A name that is not a saved key is tried in the address book
('rail0 aliases').

--chain picks the chain by slug (arc, base, …) or id, resolved by the current
environment's gateway ('rail0 env'), which also publishes the chain's keyless RPC
endpoints: they are tried in order, each with a timeout, until one answers. Without
--chain the configured default chain applies ('rail0 config set chain'). --rpc-url
reads from that node instead (its eth_chainId names the chain; --chain is then
optional and, if passed, must agree).

Default output is the native balance, in wei and in units (18 decimals, symbol from
the chain). NOTE: on Arc the native gas token is USDC with 18 decimals, while the
USDC ERC-20 (--token USDC) uses 6 — the same money, two different scales.

--token reads one ERC-20: a symbol (USDC) from the gateway's token list for the
chain, or a 0x address, whose symbol() and decimals() are read on-chain. --tokens
reads the native balance plus every active rail0 token of the chain.

-q prints the raw value in wei / base units; with several assets, one
"<asset> <symbol> <value>" line each (asset = native or the token address — on Arc
the native coin and the ERC-20 are both USDC).`,
	Example: `  rail0 keys balance 0xABCD… --chain arc
  rail0 keys balance @payer --chain arc --token USDC
  rail0 keys balance payer --tokens
  rail0 keys balance 0xABCD… --token 0x3600000000000000000000000000000000000000
  rail0 keys balance 0xABCD… --rpc-url https://rpc.example.org -q`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeKeyNames,
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		holder, keyName, err := balanceHolder(args[0])
		if err != nil {
			return err
		}
		chain, reader, err := balanceReader(ctx)
		if err != nil {
			return err
		}
		defer reader.Close()

		view := &keyBalanceView{Address: holder.Hex(), Key: keyName, ChainID: chain.ChainID, Chain: chain.Slug, ChainName: chain.Name}
		fetch := func() (any, error) {
			rows, err := readBalances(ctx, reader, chain, holder)
			if err != nil {
				return nil, err
			}
			view.Balances = rows
			return view, nil
		}
		if ui.IsQuiet() && !agentMode() {
			if _, err := fetch(); err != nil {
				return err
			}
			printBalancesQuiet(view.Balances)
			return nil
		}
		return ui.RunAPI(fetch)
	},
}

// keyBalanceView is the result: who, on which chain, and one row per asset read.
type keyBalanceView struct {
	Address   string       `json:"address"`
	Key       string       `json:"key,omitempty"`
	ChainID   int          `json:"chain_id"`
	Chain     string       `json:"chain,omitempty"`
	ChainName string       `json:"chain_name,omitempty"`
	Balances  []balanceRow `json:"balances"`
}

// balanceRow is one asset's balance: "native" or the token's address, the raw value
// in wei / base units, and the same value scaled by its decimals.
type balanceRow struct {
	Asset        string `json:"asset"`
	Symbol       string `json:"symbol"`
	Decimals     int    `json:"decimals"`
	Balance      string `json:"balance"`
	BalanceHuman string `json:"balance_human"`
}

// balanceHolder resolves the positional argument to an address. A 0x address is taken
// as is. Anything else is a saved key name (the @ optional, as on --private-key):
// the address is derived from the keychain entry and only the address leaves this
// function. When no such key exists, an "@name" in the address book is accepted too,
// since that is what @name means on every address flag; the key's error is kept when
// neither matches, as keys are what this command is documented to take.
func balanceHolder(arg string) (common.Address, string, error) {
	if evmAddressPattern.MatchString(arg) {
		return common.HexToAddress(arg), "", nil
	}
	name := strings.TrimPrefix(arg, "@")
	secret, keyErr := loadKey(name)
	if keyErr == nil {
		addr, err := addressFromKey(secret)
		if err != nil {
			return common.Address{}, "", fmt.Errorf("saved key %q: %w", name, err)
		}
		return common.HexToAddress(addr), name, nil
	}
	if addr, err := resolveAlias("@" + name); err == nil && evmAddressPattern.MatchString(addr) {
		return common.HexToAddress(addr), "", nil
	}
	return common.Address{}, "", fmt.Errorf("%q is not a 0x address, a saved key or an alias: %w", arg, keyErr)
}

// balanceReader decides the chain and the RPC endpoints to read it from.
//
// With --rpc-url the node is the authority on which chain it serves (eth_chainId): a
// --chain given beside it must agree, so a URL for the wrong network fails loudly
// instead of reading a balance from somewhere else. Otherwise the chain is --chain or
// the configured default, and its endpoints are the gateway's public_rpc_urls for it.
// The catalog entry gives the slug, name and native symbol; for a --rpc-url chain the
// gateway does not list (or cannot be reached) those stay empty and the read still works.
func balanceReader(ctx context.Context) (*rail0.Blockchain, *chainread.Reader, error) {
	if keysBalanceRPCURL != "" {
		reader, err := chainread.New([]string{keysBalanceRPCURL}, chainread.DefaultTimeout)
		if err != nil {
			return nil, nil, err
		}
		id, err := reader.ChainID(ctx)
		if err != nil {
			reader.Close()
			return nil, nil, fmt.Errorf("--rpc-url: %w", err)
		}
		if !id.IsInt64() {
			reader.Close()
			return nil, nil, fmt.Errorf("--rpc-url: implausible chain id %s", id)
		}
		if keysBalanceChainID != 0 && int64(keysBalanceChainID) != id.Int64() {
			reader.Close()
			return nil, nil, newUsageError("flag_conflict",
				"--rpc-url serves chain %s, but --chain names %s", id, chainLabel(keysBalanceChainID))
		}
		chain, _ := lookupChain(ctx, id.String())
		return chain, reader, nil
	}

	chainID := keysBalanceChainID
	if chainID == 0 {
		id, _, ok, err := configuredChainID(ctx)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, errChainRequired()
		}
		chainID = id
	}
	chain, err := lookupChain(ctx, strconv.Itoa(chainID))
	if err != nil {
		return nil, nil, err
	}
	if len(chain.PublicRPCURLs) == 0 {
		if _, catErr := loadChainCatalog(ctx); catErr != nil {
			return nil, nil, fmt.Errorf("cannot fetch the chain list from %s to find the RPC endpoints of chain %d — pass --rpc-url: %w",
				resolveBaseURL(), chainID, catErr)
		}
		return nil, nil, fmt.Errorf("%s publishes no public RPC endpoint on this gateway — pass --rpc-url", chainLabel(chainID))
	}
	reader, err := chainread.New(chain.PublicRPCURLs, chainread.DefaultTimeout)
	if err != nil {
		return nil, nil, err
	}
	return chain, reader, nil
}

// readBalances reads what the flags ask for: the native balance (default, and first
// under --tokens), one token (--token), or every active rail0 token of the chain
// (--tokens). Token symbols and decimals come from the gateway's token list when the
// token is in it; a --token 0x address it does not list is described by the contract
// itself (symbol(), decimals()).
func readBalances(ctx context.Context, r *chainread.Reader, chain *rail0.Blockchain, holder common.Address) ([]balanceRow, error) {
	var rows []balanceRow
	if keysBalanceToken == "" {
		wei, err := r.NativeBalance(ctx, holder)
		if err != nil {
			return nil, fmt.Errorf("native balance: %w", err)
		}
		symbol := chain.NativeSymbol
		if symbol == "" {
			symbol = "native"
		}
		rows = append(rows, newBalanceRow("native", symbol, nativeDecimals, wei))
	}

	var tokens []rail0.Token
	switch {
	case keysBalanceTokens:
		active, err := activeTokens(ctx, chain.ChainID)
		if err != nil {
			return nil, fmt.Errorf("fetch tokens: %w", err)
		}
		tokens = active
	case keysBalanceToken != "" && !evmAddressPattern.MatchString(keysBalanceToken):
		addr, decimals, _, err := lookupTokenAddress(ctx, chain.ChainID, keysBalanceToken)
		if err != nil {
			return nil, err
		}
		tokens = []rail0.Token{{Address: rail0.Address(addr), Decimals: decimals, Symbol: strings.ToUpper(keysBalanceToken), ChainID: chain.ChainID}}
	case keysBalanceToken != "":
		t := rail0.Token{Address: rail0.Address(keysBalanceToken), ChainID: chain.ChainID, Decimals: -1}
		if catalog, err := tokenCatalog(ctx); err == nil {
			for _, c := range catalog {
				if c.ChainID == chain.ChainID && strings.EqualFold(string(c.Address), keysBalanceToken) {
					t = c
					break
				}
			}
		}
		if t.Decimals < 0 || t.Symbol == "" {
			symbol, decimals, err := r.TokenMeta(ctx, common.HexToAddress(keysBalanceToken))
			if err != nil {
				return nil, err
			}
			t.Symbol, t.Decimals = symbol, int(decimals)
		}
		tokens = []rail0.Token{t}
	}

	for _, t := range tokens {
		units, err := r.TokenBalance(ctx, common.HexToAddress(string(t.Address)), holder)
		if err != nil {
			return nil, fmt.Errorf("%s balance: %w", t.Symbol, err)
		}
		rows = append(rows, newBalanceRow(common.HexToAddress(string(t.Address)).Hex(), t.Symbol, t.Decimals, units))
	}
	return rows, nil
}

func newBalanceRow(asset, symbol string, decimals int, value *big.Int) balanceRow {
	raw := value.String()
	return balanceRow{Asset: asset, Symbol: symbol, Decimals: decimals, Balance: raw,
		BalanceHuman: rail0.FormatAmount(raw, decimals) + " " + symbol}
}

// printBalancesQuiet is -q: the raw value alone for one asset (so `$(rail0 keys
// balance … -q)` is a number), one "<asset> <symbol> <value>" line per asset for
// several. The asset column is what tells them apart: on Arc the native coin and the
// ERC-20 are both "USDC".
func printBalancesQuiet(rows []balanceRow) {
	if len(rows) == 1 {
		fmt.Println(rows[0].Balance)
		return
	}
	for _, r := range rows {
		fmt.Printf("%s %s %s\n", r.Asset, r.Symbol, r.Balance)
	}
}

func init() {
	addChainFlag(keysBalanceCmd, &keysBalanceChainID, "c", "Chain to read (default: 'rail0 config' chain)")
	f := keysBalanceCmd.Flags()
	f.StringVar(&keysBalanceRPCURL, "rpc-url", "", "Read from this JSON-RPC endpoint instead of the gateway's public ones (its eth_chainId names the chain)")
	f.StringVarP(&keysBalanceToken, "token", "t", "", "ERC-20 to read: a symbol from 'rail0 tokens' (USDC) or a 0x token address")
	f.BoolVar(&keysBalanceTokens, "tokens", false, "Read the native balance plus every active rail0 token of the chain")
	keysBalanceCmd.MarkFlagsMutuallyExclusive("token", "tokens")
	keysCmd.AddCommand(keysBalanceCmd)
}
