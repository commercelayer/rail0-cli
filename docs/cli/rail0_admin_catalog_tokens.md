## rail0 admin catalog tokens

Every token configured in this environment, on chains not in service too

### Synopsis

List every token this gateway is configured with, whole rows, with the EIP-712 domain
(eip712_name, eip712_version) the gateway signs against. 'rail0 tokens' stops at the
chains in service.

A row names its chain by the catalogue's own reference (blockchain_id); the command adds
chain_id and chain_slug, read from 'rail0 admin catalog chains', so a row can be read by
itself.

```
rail0 admin catalog tokens [flags]
```

### Examples

```
  rail0 admin catalog tokens --pretty
```

### Options

```
  -h, --help   help for tokens
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

