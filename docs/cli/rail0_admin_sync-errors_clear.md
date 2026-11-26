## rail0 admin sync-errors clear

Delete the sync errors the filters select (write mode, audited)

### Synopsis

Delete rows of the sync-error ledger: exactly the ones the same filters list with
'rail0 admin sync-errors'. The command counts them first and asks before deleting;
--yes skips the question. It cannot be undone, and the gateway audits it with its
filters and the number of rows removed.

At least one filter is required. --all empties the whole ledger instead, and that
includes the balance_divergence rows whose count 'rail0 admin health' reports: to drop a
backlog and keep those, filter by --reason.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin sync-errors clear [flags]
```

### Examples

```
  rail0 admin sync-errors clear --reason not_found
  rail0 admin sync-errors clear --payment 0xrail0Id… --yes
  rail0 admin sync-errors clear --all
```

### Options

```
      --all              Empty the whole ledger (no filter), balance_divergence rows included
  -h, --help             help for clear
      --outcome string   Only this kind of callback: confirmed or failed
      --payment string   Only rows about this payment: its id (UUID) or its rail0_id (0x…)
      --reason string    Only this rejection reason: not_found, external_operation, chain_mismatch, payment_mismatch, operation_mismatch, unapplicable, balance_divergence, impossible_balances
      --tx-hash string   Only rows for this transaction hash
  -y, --yes              Skip the confirmation prompt (required to run non-interactively)
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

* [rail0 admin sync-errors](rail0_admin_sync-errors.md)	 - Why an indexer callback could not be applied (lists when run bare)

