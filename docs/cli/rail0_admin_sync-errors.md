## rail0 admin sync-errors

Why an indexer callback could not be applied (lists when run bare)

### Synopsis

List the gateway's sync-error ledger, newest first: each row is an on-chain event the
indexer reported and the gateway could not apply, with the reason.

  not_found            no payment has that on-chain id (an operation made outside the gateway)
  external_operation   the payment exists, the operation was not prepared by the gateway
  chain_mismatch, payment_mismatch, operation_mismatch
                       the event contradicts what the gateway holds
  unapplicable         the payment is in a state the event cannot follow
  balance_divergence, impossible_balances
                       the amounts reported do not add up; 'rail0 admin health' counts these

The ledger is never pruned by itself: 'rail0 admin sync-errors clear' removes rows.

```
rail0 admin sync-errors [flags]
```

### Examples

```
  rail0 admin sync-errors
  rail0 admin sync-errors --reason payment_mismatch --pretty
  rail0 admin sync-errors --payment 0xrail0Id…
  rail0 admin sync-errors --tx-hash 0xed7c607d…
```

### Options

```
  -h, --help             help for sync-errors
      --outcome string   Only this kind of callback: confirmed or failed
      --page int         Page number (1-based)
      --payment string   Only rows about this payment: its id (UUID) or its rail0_id (0x…)
      --per-page int     Results per page
      --reason string    Only this rejection reason: not_found, external_operation, chain_mismatch, payment_mismatch, operation_mismatch, unapplicable, balance_divergence, impossible_balances
      --sort string      Sort by created_at or reason (prefix - for descending; default: newest first)
      --tx-hash string   Only rows for this transaction hash
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, stuck transactions, sync errors, write mode
* [rail0 admin sync-errors clear](rail0_admin_sync-errors_clear.md)	 - Delete the sync errors the filters select (write mode, audited)

