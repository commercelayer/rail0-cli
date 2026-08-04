## rail0 config

Manage persisted CLI defaults (base-url, chain-id, token, output, http-timeout)

### Synopsis

Manage persisted CLI defaults stored in the config file.

Run bare to list every setting with its effective value and where it comes from.
Resolution precedence is: flag  >  env var  >  config file  >  built-in default.

A stored default fills in the matching flag when you omit it, so the chain and the
token can be set once instead of on every command. 'rail0 env <name>' sets both in
one shot along with the gateway.

Addresses and signing keys are NOT settable here: use the alias book ('rail0 aliases
add') and the OS keychain ('rail0 keys add', passed as @name), which already spare
you the retyping without making the parties or the signer implicit.

```
rail0 config [flags]
```

### Examples

```
  rail0 config
  rail0 config set base-url http://localhost:9292
  rail0 config set chain-id 5042002
  rail0 config set token USDC
  rail0 config get chain-id
  rail0 config unset chain-id
```

### Options

```
  -h, --help   help for config
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
* [rail0 config get](rail0_config_get.md)	 - Show a setting's effective value and origin
* [rail0 config path](rail0_config_path.md)	 - Print the config file path
* [rail0 config set](rail0_config_set.md)	 - Persist a setting to the config file
* [rail0 config unset](rail0_config_unset.md)	 - Remove a setting from the config file

