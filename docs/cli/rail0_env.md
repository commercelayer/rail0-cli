## rail0 env

Switch the active environment (development/staging/production) in one shot

### Synopsis

Switch a bundle of CLI settings at once by selecting an environment.

Run bare to list the environments and their parameters, with the current one
marked. Pass a name to switch: it writes that environment's settings (today just
base-url) to the config file, so every command uses them.

A stored value is still the lowest-priority source, so --base-url or
RAIL0_BASE_URL override it for a single command.

```
rail0 env [name] [flags]
```

### Examples

```
  rail0 env                 # list environments, marking the current one
  rail0 env staging         # switch to staging
  rail0 env current         # show the active environment
  rail0 env current -d      # ...with every parameter's effective value
```

### Options

```
  -h, --help   help for env
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
* [rail0 env current](rail0_env_current.md)	 - Show the active environment (--details for every parameter)

