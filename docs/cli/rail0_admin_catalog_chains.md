## rail0 admin catalog chains

Every blockchain configured in this environment, the ones not in service included

### Synopsis

List every blockchain this gateway is configured with, whole rows: 'active' says
whether it is in service, 'rpc_urls' are the endpoints the gateway itself calls.

'rail0 chains' lists only the chains a payment can be made on. A chain here and not
there is configured and not serving: switched off, or missing a token or a deployment
('rail0 admin catalog tokens', 'rail0 admin catalog contracts').

A keyed RPC endpoint is shown as its template, with a {RAIL0_RPC_KEY_<NAME>} placeholder:
the key is in the gateway's environment, never in this answer.

```
rail0 admin catalog chains [flags]
```

### Examples

```
  rail0 admin catalog chains --pretty
```

### Options

```
  -h, --help   help for chains
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

