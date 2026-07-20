## rail0 env current

Show the active environment (--details for every parameter)

```
rail0 env current [flags]
```

### Examples

```
  rail0 env current
  rail0 env current --details
```

### Options

```
  -h, --help   help for current
```

### Options inherited from parent commands

```
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
  -d, --details           Also print each parameter's effective value and origin
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 env](rail0_env.md)	 - Switch the active environment (development/staging/production) in one shot

