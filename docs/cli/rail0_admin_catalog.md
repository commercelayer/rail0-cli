## rail0 admin catalog

The chain catalogue: what a reload would change, the reload, chains, tokens, contracts (shows the diff when run bare)

### Synopsis

The gateway reads its chain configuration once, when it starts, into memory. After a
change in the database (a chain activated, a token retired, an RPC endpoint corrected)
the running process keeps serving what it read until it is restarted or reloaded.

Run bare, this compares the snapshot in memory with the database: 'stale' and 'pending'
say whether and how much they differ, and blockchains, tokens and contracts each list the
rows added, removed and changed, a changed row with its fields as from → to.

ONE PROCESS: the answer is about the process that served the request. A worker, or
another web machine, has a snapshot of its own that this cannot see.

```
rail0 admin catalog [flags]
```

### Examples

```
  rail0 admin catalog                    # is the snapshot stale, and what would change
  rail0 admin catalog reload             # apply it (write mode)
  rail0 admin catalog chains --pretty    # every chain, the ones not in service included
  rail0 admin catalog tokens --pretty
  rail0 admin catalog contracts --pretty
```

### Options

```
  -h, --help   help for catalog
```

### Options inherited from parent commands

```
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, payouts, operators, audit, configuration, catalogue, write mode
* [rail0 admin catalog chains](rail0_admin_catalog_chains.md)	 - Every blockchain configured in this environment, the ones not in service included
* [rail0 admin catalog contracts](rail0_admin_catalog_contracts.md)	 - Every RAIL0 deployment known to this environment that is not archived
* [rail0 admin catalog reload](rail0_admin_catalog_reload.md)	 - Rebuild the catalogue snapshot from the database, in the process that answers (write mode, audited)
* [rail0 admin catalog tokens](rail0_admin_catalog_tokens.md)	 - Every token configured in this environment, on chains not in service too

