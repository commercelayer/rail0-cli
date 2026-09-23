## rail0 payments list

List payments for the authenticated wallet (requires login)

### Synopsis

Returns all payments where your wallet is payer or payee.
Requires authentication — run 'rail0 auth login -p 0xKey…' first.

Filter with --status, --mode, --payer, --payee, --token, --chain-id, --disputed,
--min-amount / --max-amount, and --created-from / --created-to (ISO-8601).

--min-amount / --max-amount are DECIMALS in token units ('1.00'), like every other
amount flag. Converting them to the base units the gateway filters on needs the
token's decimals, so both --chain-id and --token are required with them — and an
amount range only means something within one token anyway.

```
rail0 payments list [flags]
```

### Examples

```
  rail0 payments list
  rail0 payments list --sort -created_at
  rail0 payments list --status authorized --mode charge
  rail0 payments list --chain-id 5042002 --disputed
  rail0 payments list --chain-id 5042002 --token 0x3600… --min-amount 1.00 --created-from 2026-01-01T00:00:00Z
```

### Options

```
      --all                   Fetch every page (ignores --page)
      --chain-id int          Filter by EVM chain ID
      --created-from string   Only payments created at/after this ISO-8601 time
      --created-to string     Only payments created at/before this ISO-8601 time
      --disputed              Filter by whether an open dispute exists
  -h, --help                  help for list
      --max-amount string     Maximum amount as a decimal in token units (inclusive); needs --chain-id and --token
      --min-amount string     Minimum amount as a decimal in token units (inclusive); needs --chain-id and --token
      --mode string           Filter by mode (authorize or charge)
      --page int              Page number (1-based)
      --payee string          Filter by payee address
      --payer string          Filter by payer address
      --per-page int          Results per page
      --sort string           Sort fields (e.g. -created_at,amount)
      --status string         Filter by payment status (e.g. authorized, captured, refunded)
      --token string          Filter by token address
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

