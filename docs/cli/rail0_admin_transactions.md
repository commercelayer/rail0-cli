## rail0 admin transactions

Transactions across every account, or only the stuck ones

### Synopsis

List transactions across every account, each with the payment it belongs to.

--stuck keeps the ones that need attention: 'unbroadcast' (signed, never sent to the
chain), 'unconfirmed' (sent, never confirmed) or 'any'. --hash finds the transaction an
explorer link or a merchant's message names. A stuck broadcast is re-sent with
'rail0 payments redrive <payment> <transaction>' (in write mode, for an operator).

```
rail0 admin transactions [flags]
```

### Examples

```
  rail0 admin transactions --stuck any
  rail0 admin transactions --hash 0xed7c607d…
  rail0 admin transactions --account 019e748b-… --status failed --chain arc
  rail0 admin transactions --operation refund --sort -created_at --pretty
```

### Options

```
      --account string     Only transactions on payments whose payee is one of this account's wallets (account id)
      --chain chain        Only transactions on this chain: a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
      --hash string        The transaction with this hash (0x + 64 hex)
  -h, --help               help for transactions
      --operation string   Filter by operation: authorize, capture, charge, void, release, refund, …
      --page int           Page number (1-based)
      --per-page int       Results per page
      --sort string        Sort fields (prefix - for descending, comma-separated), e.g. -created_at
      --status strings     Filter by status (comma-separated or repeated): pending, submitting, submitted, confirmed, failed
      --stuck string       Only stuck transactions: unbroadcast, unconfirmed or any
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, payouts, operators, audit, configuration, catalogue, write mode

