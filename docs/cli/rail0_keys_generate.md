## rail0 keys generate

Create a new wallet key pair locally and print its address and private key

### Synopsis

Create a new wallet (a secp256k1 key pair) on this machine and print its
EIP-55 address and its private key (0x-prefixed hex).

Fully offline: nothing is sent to the gateway or anywhere else, and nothing is
written to disk — not the config file, not the OS keychain. The private key
exists only in this command's output, and it is shown ONCE: it cannot be
recovered by RAIL0 or by anyone else. Store it now in a password manager (and,
to sign with it in a browser, import it into a wallet you control). Anyone who
has it controls the funds of the address.

The address and the key go to stdout (a JSON object with --json); the warning
goes to stderr, so piping or redirecting stdout keeps clean JSON. Mind where
stdout goes: a redirected file, a CI log, shell scrollback or a shared screen
all end up holding the key.

Next steps:
  rail0 keys add <name>     save it in the OS keychain for -p @<name> (prompts hidden)
  then register the ADDRESS (never the key) with RAIL0: sign up as a merchant
  (/signup in the admin) or ask an operator for an invite, and claim the account
  by signing with this wallet. Adding it to an existing account
  ('wallets create') is currently unavailable.

```
rail0 keys generate [flags]
```

### Examples

```
  rail0 keys generate
  rail0 keys generate --json
```

### Options

```
  -h, --help   help for generate
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

* [rail0 keys](rail0_keys.md)	 - Generate signing keys locally and manage them in the OS keychain

