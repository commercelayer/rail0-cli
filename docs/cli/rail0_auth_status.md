## rail0 auth status

Show the current authentication status

### Synopsis

Report whether this gateway has a usable session, and where it is stored.

Sessions are per gateway, in a file named after a digest of the base URL, so
'token_path' is the only way to know which file backs the current session —
useful when several environments are in play, or to remove one by hand.

```
rail0 auth status [flags]
```

### Options

```
  -h, --help   help for status
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

* [rail0 auth](rail0_auth.md)	 - Authenticate with the RAIL0 gateway using SIWE

