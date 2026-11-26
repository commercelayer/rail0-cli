## rail0 admin disputes

Every dispute across accounts, open and closed

### Synopsis

List the disputes of every account, newest first, each with the payment it is about
(ids, chain, payer, payee, amount) and both reasons read from the dictionary.

--status open is what is being contested right now. --reason takes one of the codes a
payer can open a dispute with ('rail0 disputes reasons'); --from and --to bound when the
dispute was opened.

Read-only: a dispute is opened and closed by the payer, on chain, and a full refund
closes it. 'rail0 admin trace <payment>' shows one payment's disputes among everything
else that happened to it.

```
rail0 admin disputes [flags]
```

### Examples

```
  rail0 admin disputes --status open
  rail0 admin disputes --status open --chain arc --pretty
  rail0 admin disputes --reason not_received --from 2026-11-01T00:00:00Z
  rail0 admin disputes --payment 0xrail0Id…
```

### Options

```
      --chain chain      Only disputes on payments of this chain: a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
      --from string      Only disputes opened at/after this ISO-8601 time
  -h, --help             help for disputes
      --page int         Page number (1-based)
      --payment string   Only this payment's disputes: its id (UUID) or its rail0_id (0x…)
      --per-page int     Results per page
      --reason string    Only disputes opened with this reason code (see 'rail0 disputes reasons')
      --sort string      Sort by opened_at or closed_at (prefix - for descending; default: newest first)
      --status string    Only disputes that are open or closed
      --to string        Only disputes opened at/before this ISO-8601 time
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

