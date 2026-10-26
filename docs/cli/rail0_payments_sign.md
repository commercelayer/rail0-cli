## rail0 payments sign

Deposit the payer signature on an already-created payment

### Synopsis

Sign an existing unsigned payment with the payer's key.

'payments create' creates AND signs in one step. This command is the recovery
path for when that second half didn't happen: the payment exists at status
'unsigned' but carries no signature, because the local signing or the submit
failed (a lost connection, a rate limit, the wrong key). Without it the payment
would be stranded — the only way forward would be creating a duplicate.

The payer signs locally: the key never leaves this machine, only the signature
is sent. Requires the payer's own key, since the gateway checks that the
recovered signer is the payment's payer.

```
rail0 payments sign <rail0Id> [flags]
```

### Examples

```
  rail0 payments sign 0xabc123… -p @buyer-key
```

### Options

```
  -h, --help                 help for sign
  -p, --private-key string   Payer's secp256k1 private key (0x-prefixed hex); signs locally, the key never leaves this machine
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

