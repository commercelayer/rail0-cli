## rail0 auth revoke-all

End EVERY session of an address, on every machine (for a key you no longer trust)

### Synopsis

Revoke every session the gateway has issued to an address — the ones on this
machine, on other machines, and any a thief is holding.

'auth logout' cannot do this: it revokes ONE token, the one stored here. This is
per ADDRESS. The gateway records a cutoff instant, and every session of the
address issued before it is refused from then on, including this machine's.

It is authorized by a fresh signature of the address's KEY, not by the session:
whoever reacts to a leaked key holds the wallet, not the stolen token. The signed
message says "Sign out of RAIL0 everywhere", so a login signature cannot be
replayed to trigger it. The key is resolved like every signing command (-p as
0x hex, @name for a keychain key, or RAIL0_PRIVATE_KEY).

Afterwards, sign in again with 'rail0 auth login'. If you suspect the key itself
leaked, this ends the sessions but does not make the key safe: move the funds
and stop using it.

```
rail0 auth revoke-all [flags]
```

### Examples

```
  rail0 auth revoke-all -p @merchant-key
```

### Options

```
  -h, --help                 help for revoke-all
  -p, --private-key string   Key OF the address whose sessions to end: 0x hex, @name for an OS-keychain key, or RAIL0_PRIVATE_KEY
  -y, --yes                  Skip the confirmation prompt (required to run non-interactively)
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

* [rail0 auth](rail0_auth.md)	 - Authenticate with the RAIL0 gateway using SIWE

