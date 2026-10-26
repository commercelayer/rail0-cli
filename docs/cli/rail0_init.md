## rail0 init

Guided first-run setup: environment, key, sign-in

### Synopsis

Walk through the setup a first payment needs, in one command:

  1. choose an environment (writes base-url, so every later command uses it)
  2. check the gateway answers
  3. optionally save a signing key in the OS keychain (referenced later as @name)
  4. optionally sign in with it (SIWE), caching a session for that gateway

Each step can be skipped, so it is safe to re-run on a machine that is already
half-configured. Nothing here is irreversible: the same steps are 'rail0 env',
'rail0 health', 'rail0 keys add' and 'rail0 auth login', and the setup can be
undone with 'rail0 keys rm' / 'rail0 auth logout'.

Interactive by design — it asks questions. In a script, run those four commands
directly instead.

```
rail0 init [flags]
```

### Examples

```
  rail0 init
```

### Options

```
  -h, --help   help for init
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

