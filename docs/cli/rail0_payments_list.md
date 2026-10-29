## rail0 payments list

List payments for the authenticated wallet (requires login)

### Synopsis

Returns all payments where your wallet is payer or payee.
Requires authentication — run 'rail0 auth login -p 0xKey…' first.

Filter with --status, --mode, --payer, --payee, --token, --chain-id, --disputed,
--operation, --rail0-id, --reference / --reference-origin, --min-amount / --max-amount, and --created-from /
--created-to (ISO-8601). --operation keeps payments carrying at least one
transaction of that operation — '--operation dispute' finds every payment ever
disputed, which --disputed (an OPEN dispute) does not.

--status takes several values, repeated or comma-separated, and matches ANY of
them: '--status authorized,partially_captured' is every payment with escrow still
capturable. Each value is checked against the payment-status vocabulary before
any request.

--min-amount / --max-amount are DECIMALS in token units ('1.00'), like every other
amount flag. Converting them to the base units the gateway filters on needs the
token's decimals, so both --chain-id and --token are required with them — and an
amount range only means something within one token anyway.

'--scope all' is the operator's list: every payment on the gateway, with the same
filters, and it needs an admin session (403 otherwise — never silently narrowed to
your own). Three filters exist only there and are refused without it: --account
(payments whose payee is one of that account's wallets), --has-error (payments
carrying a last_error_code, or --has-error=false for the clean ones) and
--expiring-within (authorized payments whose hold lapses within that duration,
'24h' or '90m', up to 90 days).

--csv writes the result as CSV instead of JSON (one page, or every page with --all):
the admin's export columns plus the two deadlines, amounts both in base units and as
decimals.

```
rail0 payments list [flags]
```

### Examples

```
  rail0 payments list
  rail0 payments list --sort -created_at
  rail0 payments list --status authorized --mode charge
  rail0 payments list --status authorized,partially_captured
  rail0 payments list --chain-id 5042002 --disputed
  rail0 payments list --all --csv > payments.csv          # every matching payment, as CSV
  rail0 payments list --chain-id 5042002 --token 0x3600… --min-amount 1.00 --created-from 2026-01-01T00:00:00Z
  rail0 payments list --scope all --has-error                # admin: every payment with an error, any account
  rail0 payments list --scope all --expiring-within 24h      # admin: holds lapsing within a day
  rail0 payments list --scope all --account 019e748b-…       # admin: one merchant's incoming payments
```

### Options

```
      --account string             With --scope all: payments whose payee is one of this account's wallets
      --all                        Fetch every page (ignores --page)
      --chain-id int               Filter by EVM chain ID
      --created-from string        Only payments created at/after this ISO-8601 time
      --created-to string          Only payments created at/before this ISO-8601 time
      --csv                        Write the payments as CSV instead of JSON (with --all: every page)
      --disputed                   Filter by whether an open dispute exists
      --expiring-within duration   With --scope all: authorized payments whose hold lapses within this duration (e.g. 24h; max 90 days)
      --has-error                  With --scope all: payments carrying a last_error_code (--has-error=false: none)
  -h, --help                       help for list
      --max-amount string          Maximum amount as a decimal in token units (inclusive); needs --chain-id and --token
      --min-amount string          Minimum amount as a decimal in token units (inclusive); needs --chain-id and --token
      --mode string                Filter by mode (authorize or charge)
      --operation string           Only payments with at least one transaction of this operation (e.g. dispute, refund)
      --page int                   Page number (1-based)
      --payee string               Filter by payee address
      --payer string               Filter by payer address
      --per-page int               Results per page
      --rail0-id string            Filter by the logical on-chain payment id (0x…)
      --reference string           Filter by the payment's identifier in the merchant's own system (exact)
      --reference-origin string    Filter by the system the reference comes from
      --scope string               mine (default): your wallet's payments; all: every payment on the gateway (admin only)
      --sort string                Sort fields (e.g. -created_at,amount)
      --status strings             Filter by payment status; repeat or comma-separate for any of several (e.g. authorized,partially_captured)
      --token string               Filter by token address
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

