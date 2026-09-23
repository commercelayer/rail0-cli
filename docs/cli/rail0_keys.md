## rail0 keys

Manage signing keys in the OS keychain

### Synopsis

Save signing keys in the OS keychain and reference them as "@name" on the
--private-key flag (e.g. 'rail0 payments create -p @payer'), instead of pasting
a raw key or exporting RAIL0_PRIVATE_KEY.

The secret never touches the config file or your shell history. Backends: macOS
Keychain, Windows Credential Manager, Linux Secret Service. On a system without
one (e.g. headless Linux) these commands return an error; use --private-key /
RAIL0_PRIVATE_KEY there.

There is no listing of saved keys, by design: a keychain cannot be enumerated,
so a list could only report a copy of the names kept alongside the config — and
that copy drifts. Ask about one key at a time with 'rail0 keys address <name>',
which reads the keychain itself and cannot go stale.

### Examples

```
  rail0 keys add payer
  rail0 payments create -p @payer …
  rail0 keys address payer
  rail0 keys rm payer
```

### Options

```
  -h, --help   help for keys
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
* [rail0 keys add](rail0_keys_add.md)	 - Save a signing key in the OS keychain (prompts for the key; names must be unique)
* [rail0 keys address](rail0_keys_address.md)	 - Show the EVM address derived from a saved key (the address, never the key)
* [rail0 keys rm](rail0_keys_rm.md)	 - Remove a saved signing key from the OS keychain

