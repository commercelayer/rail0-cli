## rail0 admin trace

Everything about one payment: transactions, sync errors, deliveries, disputes, operator actions

### Synopsis

Answer "what happened to this payment" in one command, whoever the payment belongs to.

The payment is read with its transactions, and then everything else that names it:
the sync errors (indexer callbacks about it that could not be applied), the webhook
deliveries across every subscription, its disputes, and what operators did to it.
'totals' says how many of each exist, in case one kind has more than a page.

--timeline prints the same facts as one chronological list instead: the payment's own
history with the sync errors, deliveries, disputes and operator actions in their place.

The argument is the payment's id (UUID) or its rail0_id (0x…, 64 hex).

```
rail0 admin trace <payment> [flags]
```

### Examples

```
  rail0 admin trace 0xrail0Id…
  rail0 admin trace 019e748b-… --timeline --pretty
```

### Options

```
  -h, --help       help for trace
      --timeline   Print one chronological list instead of the sections
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, operators, audit, write mode

