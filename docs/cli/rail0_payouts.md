## rail0 payouts

Send a wallet's tokens to your own venue (lists when run bare; requires login)

### Synopsis

Pay out: move a merchant wallet's tokens to a deposit address at a venue of your
own — an exchange account, Circle Mint, another wallet you control. Run bare to
list the account's payouts.

The gateway does the on-chain leg and nothing else. It builds an unsigned ERC-20
transfer from the wallet to a registered destination; this CLI checks it is
EXACTLY that transfer (token, recipient, amount, chain, no value) and signs it
locally with your key. The gateway never holds funds or keys and never talks to
the venue.

First payout to a new venue:
  1. payouts destinations add -c <chain> -t USDC -A 0xDeposit… --min-amount <min>
       register the deposit address; you confirm you checked the details
  2. payouts send --destination <id> --proof -p @merchant-key
       a small proof transfer (1 token unit, raised to the minimum)
  3. payouts destinations confirm <id>
       once the venue shows the proof CREDITED: the destination is verified
  4. payouts send --destination <id> --amount 250.00 -p @merchant-key
       or --amount all: the balance minus the refund reserve

Minimum: a deposit below the venue's minimum may be ignored by the venue and lost
for good. Every destination states it (--min-amount, or --no-minimum when the venue
has none): payouts below it are refused.

Refund reserve: '--amount all' keeps the destination's reserve in the wallet, so
refunds still inside their window can be paid (default: the gateway's suggestion,
the refundable balance of the wallet's payments in that token).

A payout goes pending → submitted → confirmed (or failed: its error_code says
why). One pending payout per destination. A deactivated account is read-only: it
cannot prepare, send or change payouts or destinations (403 account_deactivated).

Every read covers the whole account; writes act for the wallet you sign in with,
which must be the destination's.

```
rail0 payouts [flags]
```

### Examples

```
  rail0 payouts
  rail0 payouts --status submitted,confirmed --purpose payout
  rail0 payouts send --destination <destination_id> --amount 100.00 -p @merchant-key
  rail0 payouts get <payout_id>
```

### Options

```
      --created-from string   Only payouts created at/after this ISO-8601 time
      --created-to string     Only payouts created at/before this ISO-8601 time
      --destination string    Only payouts to this destination (destination id)
  -h, --help                  help for payouts
      --page int              Page number (1-based)
      --per-page int          Payouts per page (max 100)
      --purpose string        Filter by purpose: payout or proof
      --sort string           Sort fields (e.g. -created_at)
      --status strings        Filter by status (pending, submitted, confirmed, failed); repeat or comma-separate for any of several
      --wallet string         Only payouts from this wallet (wallet id)
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 payouts cancel](rail0_payouts_cancel.md)	 - Cancel a pending payout that was never signed
* [rail0 payouts destinations](rail0_payouts_destinations.md)	 - Manage where payouts go (lists when run bare)
* [rail0 payouts events](rail0_payouts_events.md)	 - A payout's audit trail, newest first
* [rail0 payouts get](rail0_payouts_get.md)	 - Fetch one payout
* [rail0 payouts list](rail0_payouts_list.md)	 - List the account's payouts, newest first
* [rail0 payouts prepare](rail0_payouts_prepare.md)	 - Prepare a payout without signing it (shows the exact amount)
* [rail0 payouts report-hash](rail0_payouts_report-hash.md)	 - Report a transfer you broadcast yourself
* [rail0 payouts resend](rail0_payouts_resend.md)	 - Re-broadcast the signed transfer a submit stored
* [rail0 payouts send](rail0_payouts_send.md)	 - Prepare, verify, sign and submit a payout in one step
* [rail0 payouts submit](rail0_payouts_submit.md)	 - Verify, sign and submit a prepared payout

