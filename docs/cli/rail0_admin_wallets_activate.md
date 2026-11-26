## rail0 admin wallets activate

Switch a wallet back on, on any account (write mode, audited)

### Synopsis

Switch a wallet back on: it is offered to buyers again and its webhooks resume. The
only way back for a wallet an operator switched off. No confirmation is asked:
'rail0 admin wallets deactivate' reverses it.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin wallets activate <wallet> [flags]
```

### Examples

```
  rail0 admin wallets activate 0x2D2B… --account 019e748b-…
```

### Options

```
  -h, --help   help for activate
```

### Options inherited from parent commands

```
      --account string    The account the wallets belong to (required)
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 admin wallets](rail0_admin_wallets.md)	 - Any account's wallets: list them, see what names one, switch it off or on (lists when run bare)

