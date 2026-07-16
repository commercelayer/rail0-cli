## rail0 keys

Manage signing keys in the OS keychain (lists names when run bare)

### Synopsis

Save signing keys in the OS keychain and reference them as "@name" on the
--private-key flag (e.g. 'rail0 payments create -p @payer'), instead of pasting
a raw key or exporting RAIL0_PRIVATE_KEY.

The secret never touches the config file or your shell history — only the key
names are tracked locally so they can be listed. Backends: macOS Keychain,
Windows Credential Manager, Linux Secret Service. On a system without one (e.g.
headless Linux) these commands return an error; use --private-key /
RAIL0_PRIVATE_KEY there.

```
rail0 keys [flags]
```

### Examples

```
  rail0 keys add payer
  rail0 payments create -p @payer …
  rail0 keys
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

