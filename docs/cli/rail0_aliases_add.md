## rail0 aliases add

Add an address alias (names must be unique — rm to replace)

```
rail0 aliases add <name> <address> [flags]
```

### Examples

```
  rail0 aliases add payee 0xAbC123…
```

### Options

```
  -h, --help   help for add
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

* [rail0 aliases](rail0_aliases.md)	 - Address book — list saved addresses, or manage them with add/rm

