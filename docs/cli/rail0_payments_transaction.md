## rail0 payments transaction

Fetch one of a payment's transactions

### Synopsis

Fetch a single transaction of a payment by its id — the id every lifecycle
command prints when the gateway accepts an operation (202).

The direct lookup for "what happened to THAT attempt": its status, the decoded
revert reason of a failed one, and whether a stuck one is 'redrivable'.
Readable by either participant.

```
rail0 payments transaction <rail0Id> <transaction_id> [flags]
```

### Examples

```
  rail0 payments transaction 0xabc123… 7f1c0c8e-…
```

### Options

```
  -h, --help   help for transaction
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

