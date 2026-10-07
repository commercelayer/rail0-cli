## rail0 payments update

Change a payment's description

### Synopsis

Change the description of a payment — the one field that can be edited after
create. Either party (payer or payee) may do it, in any status, closed ones
included.

The description lives only on the gateway: it is in nothing signed or hashed
on-chain, so this moves no money, needs no signing key and sends no webhook.
At most 255 characters.

Pass the new text with --description, or --clear to remove it (exactly one).

Requires login — run 'rail0 auth login -p 0xKey…' first.

```
rail0 payments update <rail0Id> [flags]
```

### Examples

```
  rail0 payments update 0xabc123… -d "order #42"
  rail0 payments update 0xabc123… --clear
```

### Options

```
      --clear                Remove the description
  -d, --description string   New description (max 255 characters)
  -h, --help                 help for update
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

* [rail0 payments](rail0_payments.md)	 - Payment lifecycle operations

