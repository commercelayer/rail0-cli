package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	rail0 "github.com/commercelayer/rail0-go"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	chainsSymbol      string
	chainsNetworkType string
)

var chainsCmd = &cobra.Command{
	Use:     "chains",
	Aliases: []string{"blockchains"},
	Short:   "List supported blockchains (chain IDs used in --chain-id)",
	Long: `List the blockchains supported by the environment.

Filter with --symbol (the chain's native symbol, e.g. ETH / USDC / CELO) and/or
--network-type (testnet | mainnet). Both match case-insensitively; combine them
to narrow further.

Each chain carries 'settlement': how long an operation has taken to settle on
this gateway, broadcast to confirmation, as p50/p90 over a trailing window. The
percentiles are null until the chain has enough confirmed transactions to be
measured. 'settlement_human' says it in one line ("p50 11s · p90 19s …", or
"n/a (3 samples)"). The p90 is also what sizes the default --wait deadline.`,
	Example: `  rail0 chains
  rail0 chains --symbol USDC
  rail0 chains --network-type testnet
  rail0 chains -s ETH -n mainnet`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		networkType := strings.ToLower(chainsNetworkType)
		if networkType != "" && networkType != "testnet" && networkType != "mainnet" {
			return fmt.Errorf("invalid --network-type %q: must be 'testnet' or 'mainnet'", chainsNetworkType)
		}
		return ui.RunAPI(func() (any, error) {
			chains, err := newReadClient().Chains.List(context.Background(),
				rail0.ListChainsParams{NetworkType: networkType, Symbol: chainsSymbol})
			if err != nil {
				return nil, err
			}
			out := make([]chainView, len(chains))
			for i := range chains {
				out[i] = chainView{Blockchain: chains[i], SettlementHuman: settlementLabel(chains[i].Settlement)}
			}
			return out, nil
		})
	},
}

// chainView is a chain plus its settlement in words. Embedded, so the JSON is the
// gateway's chain with one extra key — the raw `settlement` object stays for scripts,
// the companion is for the eye (the same pattern as paymentView's *_human fields).
type chainView struct {
	rail0.Blockchain
	SettlementHuman string `json:"settlement_human,omitempty"`
}

// settlementLabel renders a chain's measured settlement (rail0-gateway#365) as
// "p50 11s · p90 19s (20 samples, 7d)", or "n/a (N samples)" while the percentiles are
// null — below the gateway's minimum sample. The sample count is kept in both forms:
// it is what tells "not measured yet" apart from "measured and fast".
func settlementLabel(st rail0.BlockchainSettlement) string {
	if st.P50Seconds == nil || st.P90Seconds == nil {
		return fmt.Sprintf("n/a (%d samples)", st.SampleSize)
	}
	secs := func(n int) string { return (time.Duration(n) * time.Second).String() }
	label := fmt.Sprintf("p50 %s · p90 %s (%d samples", secs(*st.P50Seconds), secs(*st.P90Seconds), st.SampleSize)
	if st.WindowDays > 0 {
		label += fmt.Sprintf(", %dd", st.WindowDays)
	}
	return label + ")"
}

var (
	chainCatalogOnce sync.Once
	chainCatalogList []rail0.Blockchain
)

// chainCatalog returns the environment's active blockchains (GET /blockchains),
// fetched at most once per process: the interactive create flow labels a chain
// in several prompts, and re-fetching the catalog for each of them would add a
// round-trip per keystroke-free prompt. Best-effort — a fetch error caches the
// empty slice, so callers degrade to the bare chain id instead of retrying (and
// stalling) against an unreachable gateway on every label.
func chainCatalog() []rail0.Blockchain {
	chainCatalogOnce.Do(func() {
		chains, err := newReadClient().Chains.List(context.Background())
		if err != nil {
			return
		}
		chainCatalogList = chains
	})
	return chainCatalogList
}

// chainLabel renders a chain id for humans, e.g. "Base Sepolia (84532)", using
// the cached catalog to resolve the name.
func chainLabel(id int) string {
	for _, c := range chainCatalog() {
		if c.ChainID == id {
			return formatChainLabel(c.Name, id)
		}
	}
	return formatChainLabel("", id)
}

// formatChainLabel is the single place the "name (id)" wording is decided; an
// unknown name (unreachable catalog, or an id outside this environment) falls
// back to "chain <id>", which still reads as a sentence in the prompts.
func formatChainLabel(name string, id int) string {
	if name == "" {
		return fmt.Sprintf("chain %d", id)
	}
	return fmt.Sprintf("%s (%d)", name, id)
}

func init() {
	chainsCmd.Flags().StringVarP(&chainsSymbol, "symbol", "s", "",
		"Filter by native symbol (e.g. ETH, USDC, CELO)")
	chainsCmd.Flags().StringVarP(&chainsNetworkType, "network-type", "n", "",
		"Filter by network type: testnet or mainnet")
}
