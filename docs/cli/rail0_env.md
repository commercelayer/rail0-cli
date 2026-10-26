## rail0 env

Switch the active environment (development/staging/production) in one shot

### Synopsis

Switch a bundle of CLI settings at once by selecting an environment.

Run bare to list the selectable environments, with the current one marked; add
--details to also print the parameters each one declares. Pass a name to switch:
it writes that environment's settings to the config file, so every command uses
them.

The bundle is base-url plus, on development and staging, chain-id and token — so
a switch moves the default chain and token as well as the gateway. 'env <name>
--details' prints what it ended up with.

After a switch the environment-managed settings are exactly the target's: a key
the target does not declare is REMOVED, so production (base-url alone) does not
inherit the chain and token of the environment you came from. A switch that would
clear something names the values and asks first; --yes skips the prompt, and
without a terminal the switch refuses rather than hanging.

A stored value is still the lowest-priority source, so --base-url or
RAIL0_BASE_URL override it for a single command — and while either one points at
a different gateway, a switch is refused: it could not take effect.

```
rail0 env [name] [flags]
```

### Examples

```
  rail0 env                 # list the environments, marking the current one
  rail0 env -d              # ...with the parameters each one declares
  rail0 env staging         # switch to staging
  rail0 env staging -d      # ...and also print the resulting parameters
  rail0 env current         # show the active environment
  rail0 env current -d      # ...with every parameter's effective value
```

### Options

```
  -d, --details   Also print each parameter's effective value and origin
  -h, --help      help for env
  -y, --yes       Skip the confirmation prompt (required to run non-interactively)
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 env current](rail0_env_current.md)	 - Show the active environment (--details for every parameter)

