## rail0 payouts destinations

Manage where payouts go (lists when run bare)

### Synopsis

Manage the account's payout destinations — the deposit addresses at a venue of
your own (an exchange account, Circle Mint, another wallet you control) that a
wallet may pay out to. Run bare to list them.

A destination is bound to ONE chain and ONE token. An EVM address is the same
string on every chain, but a venue credits only the network it issued the address
for: USDC sent on another chain to that address is usually lost. Generate the
deposit address at the venue WITH THE NETWORK SET to the destination's chain.

Its lifecycle:

  1. 'add'       register it — you confirm you checked the details at the venue.
                 It is born "unverified".
  2. proof       'rail0 payouts send --destination <id> --proof' sends a small
                 transfer (1 token unit, raised to the minimum).
  3. 'confirm'   once the venue shows the proof CREDITED: the destination
                 becomes "verified" and takes payouts of any amount.

Minimum: a venue may silently ignore a deposit below its minimum, and that deposit
can be lost for good. Every destination states it: --min-amount with the venue's
minimum, or --no-minimum when the venue has none. Payouts below it are refused, and
the proof is raised to it. Changing the minimum asks for the details confirmation
again, and raising it above the latest proof takes the verification away (the
destination needs a new proof). A destination registered before the minimum was
asked shows it as "not set" (the "minimum" field of list and get): state it with
'update'.

Reserves are decided at payout time, not on the destination: 'reserves <id>'
shows what a payout to it would keep back in the wallet right now (the refund
reserve and, on a chain whose gas is paid in this token, the gas reserve) and
what '--amount all' would move. See 'rail0 payouts send --help'.

Every destination has a label (--label), required and unique in the account,
ignoring case: two live destinations may not share one. 'delete' frees it.

'delete' removes a destination for good (it leaves the lists, its payment method
and its label are free again; its payouts and audit trail keep naming it).

Every wallet of the account can READ every destination and disable one; only the
destination's own wallet may change or re-enable it. A deactivated account is
read-only: every change is refused (403 account_deactivated).

```
rail0 payouts destinations [flags]
```

### Examples

```
  rail0 payouts destinations
  rail0 payouts destinations add -c arc -t USDC -A 0xVenueDeposit… -l "Exchange" --min-amount 10
  rail0 payouts destinations confirm <destination_id>
  rail0 payouts destinations reserves <destination_id>
  rail0 payouts destinations events <destination_id>
```

### Options

```
  -h, --help              help for destinations
      --include-deleted   List deleted destinations too (deleted_at set)
      --page int          Page number (1-based)
      --per-page int      Destinations per page
      --sort string       Sort fields (e.g. -created_at)
      --status string     Filter by status: unverified, verified, disabled
      --wallet string     Only this wallet's destinations (wallet id)
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

* [rail0 payouts](rail0_payouts.md)	 - Send a wallet's tokens to your own venue (lists when run bare; requires login)
* [rail0 payouts destinations add](rail0_payouts_destinations_add.md)	 - Register a payout destination (asks you to confirm the details)
* [rail0 payouts destinations confirm](rail0_payouts_destinations_confirm.md)	 - Attest the venue credited the proof: the destination becomes verified
* [rail0 payouts destinations delete](rail0_payouts_destinations_delete.md)	 - Delete a destination (it leaves the lists; cannot be undone)
* [rail0 payouts destinations disable](rail0_payouts_destinations_disable.md)	 - Switch a destination off (reversible)
* [rail0 payouts destinations enable](rail0_payouts_destinations_enable.md)	 - Switch a disabled destination back on
* [rail0 payouts destinations events](rail0_payouts_destinations_events.md)	 - A destination's audit trail (its events and its payouts'), newest first
* [rail0 payouts destinations get](rail0_payouts_destinations_get.md)	 - Fetch one payout destination
* [rail0 payouts destinations list](rail0_payouts_destinations_list.md)	 - List the account's payout destinations (every wallet's)
* [rail0 payouts destinations reserves](rail0_payouts_destinations_reserves.md)	 - What a payout to a destination would keep back now, and what it could move
* [rail0 payouts destinations update](rail0_payouts_destinations_update.md)	 - Change a destination's label or minimum

