## rail0 aliases

Address book — list saved addresses, or manage them with add/rm

### Synopsis

Manage a global address book stored in the config file.

Run bare to list the saved aliases. Once an alias is set, reference it as
"@name" on any address flag (--from / --to / --address) instead of pasting a
raw 0x address.

```
rail0 aliases [flags]
```

### Examples

```
  rail0 aliases
  rail0 aliases add payee 0xAbC123…
  rail0 aliases rm payee
  rail0 wallets alias 0xAbC123… payee
```

### Options

```
  -h, --help   help for aliases
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
* [rail0 aliases add](rail0_aliases_add.md)	 - Add an address alias (names must be unique — rm to replace)
* [rail0 aliases rm](rail0_aliases_rm.md)	 - Remove an address alias

