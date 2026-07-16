## rail0 config

Manage persisted CLI defaults (e.g. base-url)

### Synopsis

Manage persisted CLI defaults stored in the config file.

Run bare to list every setting with its effective value and where it comes from.
Resolution precedence is: flag  >  env var  >  config file  >  built-in default.

Only non-sensitive values are stored — never private keys or tokens.

```
rail0 config [flags]
```

### Examples

```
  rail0 config
  rail0 config set base-url http://localhost:9292
  rail0 config get base-url
  rail0 config unset base-url
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

