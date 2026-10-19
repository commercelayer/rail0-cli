## rail0 keys add

Save a signing key in the OS keychain (prompts for the key; names must be unique)

### Synopsis

Save a signing key in the OS keychain under a name, then use it as @name on
--private-key anywhere a key is taken.

The secret is PROMPTED (hidden), never taken as an argument, so it stays out of
your shell history and out of the process list. It goes to the keychain; only
the NAME is written to the config file, so the config can be copied between
machines without carrying a secret.

Because names live in the config and secrets live in the keychain, the two can
disagree — delete the config, or move it between machines, and the keychain may
still hold an entry the name index no longer lists. This command REFUSES to
write over such an orphan rather than asking yes/no: a private key cannot be
recovered, and a confirmation in that position is one people answer by reflex.
'keys rm <name>' clears it, and is the only way to, since a keychain cannot be
enumerated — which is exactly why the name index exists.

```
rail0 keys add <name> [flags]
```

### Examples

```
  rail0 keys add payer
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

* [rail0 keys](rail0_keys.md)	 - Generate signing keys locally and manage them in the OS keychain

