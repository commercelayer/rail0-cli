## rail0 payments update

Change a payment's description, internal note or reference

### Synopsis

Change the off-chain fields of a payment, in any status, closed ones included.
They live only on the gateway — in nothing signed or hashed on-chain — so this
moves no money, needs no signing key and sends no webhook.

  --description / --clear   either party; at most 255 characters. --clear
                            removes it (not both).
  --note                    the PAYEE's internal note, at most 2000 characters;
                            never shown to the payer.
  --reference               the payment's identifier in your own system (e.g.
                            an order number there), at most 128 characters.
  --reference-origin        which system that is: a lowercase slug, e.g.
                            commercelayer.

The last three are the payee's: the payer gets 403 not_the_payee. Pass one
as "" to clear it; clearing the reference clears its origin too. Only the
flags given are sent, and at least one is required.

Requires login — run 'rail0 auth login -p 0xKey…' first.

```
rail0 payments update <rail0Id> [flags]
```

### Examples

```
  rail0 payments update 0xabc123… -d "spring collection"
  rail0 payments update 0xabc123… --clear
  rail0 payments update 0xabc123… --note "refund agreed by email"
  rail0 payments update 0xabc123… --reference ORD-1042 --reference-origin commercelayer
```

### Options

```
      --clear                     Remove the description
  -d, --description string        New description (max 255 characters)
  -h, --help                      help for update
      --note string               The payee's internal note, never shown to the payer ("" clears it)
      --reference string          The payment's identifier in your own system ("" clears it and its origin)
      --reference-origin string   Which system the reference comes from: a lowercase slug, e.g. commercelayer
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

* [rail0 payments](rail0_payments.md)	 - Payment lifecycle operations

