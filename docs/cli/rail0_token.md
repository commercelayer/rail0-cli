## rail0 token

Print the stored session token, or suggest logging in

### Synopsis

Print the JWT session token saved by the last 'rail0 auth login' for the
current gateway (~/.cache/rail0/tokens/<digest of the base URL>), together with
its expiry. If no valid token is stored, it suggests logging in instead.

This is the singular session token — not to be confused with 'rail0 tokens',
which lists the gateway's accepted payment tokens.

```
rail0 token [flags]
```

### Examples

```
  rail0 token
```

### Options

```
  -h, --help   help for token
```

### Options inherited from parent commands

```
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol

