package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	rail0 "github.com/rail0/go-sdk"

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
to narrow further.`,
	Example: `  rail0 chains
  rail0 chains --symbol USDC
  rail0 chains --network-type testnet
  rail0 chains -s ETH -n mainnet`,
	RunE: func(cmd *cobra.Command, args []string) error {
		networkType := strings.ToLower(chainsNetworkType)
		if networkType != "" && networkType != "testnet" && networkType != "mainnet" {
			return fmt.Errorf("invalid --network-type %q: must be 'testnet' or 'mainnet'", chainsNetworkType)
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Chains.List(context.Background(),
				rail0.ListChainsParams{NetworkType: networkType, Symbol: chainsSymbol})
		})
	},
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
		chains, err := newClient().Chains.List(context.Background())
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
