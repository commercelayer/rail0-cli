## rail0 admin catalog contracts

Every RAIL0 deployment known to this environment that is not archived

### Synopsis

List the RAIL0 deployments this gateway knows, whole rows: address, version, the block
the indexer starts from, and whether the deployment is the active one of its chain.
Archived deployments are left out: nothing may reference one again.

Read with 'rail0 admin catalog chains': a chain with no active deployment cannot take a
payment. The command adds chain_id and chain_slug to each row, as for the tokens.

```
rail0 admin catalog contracts [flags]
```

### Examples

```
  rail0 admin catalog contracts --pretty
```

### Options

```
  -h, --help   help for contracts
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

* [rail0 admin catalog](rail0_admin_catalog.md)	 - The chain catalogue: what a reload would change, the reload, chains, tokens, contracts (shows the diff when run bare)

