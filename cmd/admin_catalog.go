package cmd

import (
	"context"
	"fmt"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// The chain catalogue as the operator reads it: what this gateway is configured to
// serve, what is not in service included, and whether the snapshot the running process
// serves from memory still matches the database.

var adminCatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "The chain catalogue: what a reload would change, the reload, chains, tokens, contracts (shows the diff when run bare)",
	Long: `The gateway reads its chain configuration once, when it starts, into memory. After a
change in the database (a chain activated, a token retired, an RPC endpoint corrected)
the running process keeps serving what it read until it is restarted or reloaded.

Run bare, this compares the snapshot in memory with the database: 'stale' and 'pending'
say whether and how much they differ, and blockchains, tokens and contracts each list the
rows added, removed and changed, a changed row with its fields as from → to.

ONE PROCESS: the answer is about the process that served the request. A worker, or
another web machine, has a snapshot of its own that this cannot see.`,
	Example: `  rail0 admin catalog                    # is the snapshot stale, and what would change
  rail0 admin catalog reload             # apply it (write mode)
  rail0 admin catalog chains --pretty    # every chain, the ones not in service included
  rail0 admin catalog tokens --pretty
  rail0 admin catalog contracts --pretty`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			diff, err := newReadClient().Admin.CatalogDiff(context.Background())
			if err != nil {
				return nil, err
			}
			return catalogDiffView{CatalogDiff: diff, Pending: diff.Pending}, nil
		})
	}),
}

// catalogDiffView prints `pending` whatever it is: 0 is the answer an operator is
// looking for ("nothing to reload"), and the SDK's type leaves a zero out.
type catalogDiffView struct {
	*rail0.CatalogDiff
	Pending int `json:"pending"`
}

var adminCatalogReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Rebuild the catalogue snapshot from the database, in the process that answers (write mode, audited)",
	Long: `Rebuild the chain catalogue's in-memory snapshot from the database. What the database
holds becomes what the process serves: a chain switched on starts taking payments, a
token retired stops being offered.

The command reads the diff first and asks, saying how many changes are pending. The
answer says what the rebuilt snapshot holds and what the reload applied.

ONE PROCESS, the one that answers the request. It is the tool for an environment served
by a single process. Where several serve (web machines, a worker) each has its own
snapshot and this reaches one of them: a restart is what reaches them all.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin catalog reload
  rail0 admin catalog reload --yes`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if err := requireWriteMode(); err != nil {
			return err
		}
		ctx := context.Background()
		diff, err := newReadClient().Admin.CatalogDiff(ctx)
		if err != nil {
			return err
		}
		if err := confirmDestructive(reloadCatalogPrompt(diff)); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Reloading…", func() (any, error) {
			return newClient().Admin.ReloadCatalog(ctx)
		})
	}),
}

// reloadCatalogPrompt is the question asked before a reload: how much it would change,
// and that it changes one process.
func reloadCatalogPrompt(diff *rail0.CatalogDiff) string {
	what := "the snapshot already matches the database: the reload changes nothing"
	if diff.Pending == 1 {
		what = "1 change is pending and becomes live ('rail0 admin catalog' lists it)"
	} else if diff.Pending > 1 {
		what = fmt.Sprintf("%d changes are pending and become live ('rail0 admin catalog' lists them)", diff.Pending)
	}
	return "Reload the chain catalogue of the process that answers?\n" +
		"  - " + what + "\n" +
		"  - one process only: other web machines and the worker keep their own snapshot until restarted"
}

var adminCatalogChainsCmd = &cobra.Command{
	Use:   "chains",
	Short: "Every blockchain configured in this environment, the ones not in service included",
	Long: `List every blockchain this gateway is configured with, whole rows: 'active' says
whether it is in service, 'rpc_urls' are the endpoints the gateway itself calls.

'rail0 chains' lists only the chains a payment can be made on. A chain here and not
there is configured and not serving: switched off, or missing a token or a deployment
('rail0 admin catalog tokens', 'rail0 admin catalog contracts').

A keyed RPC endpoint is shown as its template, with a {RAIL0_RPC_KEY_<NAME>} placeholder:
the key is in the gateway's environment, never in this answer.`,
	Example: `  rail0 admin catalog chains --pretty`,
	Args:    cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			rows, err := newReadClient().Admin.Blockchains(context.Background())
			if err != nil {
				return nil, err
			}
			return table(rows, catalogChainColumns), nil
		})
	}),
}

var adminCatalogTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Every token configured in this environment, on chains not in service too",
	Long: `List every token this gateway is configured with, whole rows, with the EIP-712 domain
(eip712_name, eip712_version) the gateway signs against. 'rail0 tokens' stops at the
chains in service.

A row names its chain by the catalogue's own reference (blockchain_id); the command adds
chain_id and chain_slug, read from 'rail0 admin catalog chains', so a row can be read by
itself.`,
	Example: `  rail0 admin catalog tokens --pretty`,
	Args:    cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			rows, err := catalogRowsWithChain(context.Background(), (*rail0.AdminService).Tokens)
			if err != nil {
				return nil, err
			}
			return table(rows, catalogTokenColumns), nil
		})
	}),
}

var adminCatalogContractsCmd = &cobra.Command{
	Use:   "contracts",
	Short: "Every RAIL0 deployment known to this environment that is not archived",
	Long: `List the RAIL0 deployments this gateway knows, whole rows: address, version, the block
the indexer starts from, and whether the deployment is the active one of its chain.
Archived deployments are left out: nothing may reference one again.

Read with 'rail0 admin catalog chains': a chain with no active deployment cannot take a
payment. The command adds chain_id and chain_slug to each row, as for the tokens.`,
	Example: `  rail0 admin catalog contracts --pretty`,
	Args:    cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			rows, err := catalogRowsWithChain(context.Background(), (*rail0.AdminService).Contracts)
			if err != nil {
				return nil, err
			}
			return table(rows, catalogContractColumns), nil
		})
	}),
}

// catalogRowsWithChain reads token or contract rows and gives each the chain_id and the
// slug of the blockchain it references, from the operator's chain list (the rows carry only the
// catalogue's internal blockchain_id). A row that already has a chain_id keeps it, and
// one whose chain is not listed is returned as it came.
func catalogRowsWithChain(ctx context.Context, list func(*rail0.AdminService, context.Context) ([]map[string]any, error)) ([]map[string]any, error) {
	client := newReadClient()
	rows, err := list(client.Admin, ctx)
	if err != nil {
		return nil, err
	}
	chains, err := client.Admin.Blockchains(ctx)
	if err != nil {
		return nil, err
	}
	chainOf := map[string]map[string]any{}
	for _, c := range chains {
		if id, _ := c["id"].(string); id != "" {
			chainOf[id] = c
		}
	}
	for _, row := range rows {
		id, _ := row["blockchain_id"].(string)
		chain := chainOf[id]
		if chain == nil {
			continue
		}
		if row["chain_id"] == nil {
			row["chain_id"] = chain["chain_id"]
		}
		// The slug too: 'rail0 chains' cannot name a chain that is not in service, and
		// those are the rows this list exists for.
		if row["chain_slug"] == nil && chain["slug"] != nil {
			row["chain_slug"] = chain["slug"]
		}
	}
	return rows, nil
}

func init() {
	addYesFlag(adminCatalogReloadCmd)
}
