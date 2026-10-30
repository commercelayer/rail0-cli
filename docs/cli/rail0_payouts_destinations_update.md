## rail0 payouts destinations update

Change a destination's label, refund reserve or minimum

### Synopsis

Change a payout destination's label, refund reserve or minimum deposit. Only the
destination's own wallet may.

--reserve and --min-amount are decimals in token units, converted with the
destination token's decimals. An empty value clears the field: --reserve ""
restores the gateway's suggestion, --min-amount "" removes the minimum.

CHANGING the minimum asks for the details confirmation again, exactly like 'add'
(a y/N prompt on a terminal, or --confirm-details; --yes does not count): the
minimum is copied from the venue, and a wrong one is how a deposit gets lost.
Re-sending the current value asks nothing. Raising it above the destination's
latest proof takes the verification away: the destination goes back to
"unverified" and needs a new proof at the new minimum.

To switch a destination off or on, use 'disable' / 'enable'.

```
rail0 payouts destinations update <destination_id> [flags]
```

### Examples

```
  rail0 payouts destinations update <destination_id> --label "Exchange (main)"
  rail0 payouts destinations update <destination_id> --reserve 50
  rail0 payouts destinations update <destination_id> --min-amount 25 --confirm-details
```

### Options

```
      --confirm-details     Accept the details statement without a prompt (needed without a TTY or with --agent); --yes does NOT imply it
  -h, --help                help for update
  -l, --label string        New label
      --min-amount string   New minimum deposit, decimal in token units ("" removes it); asks for the details confirmation
      --reserve string      New refund reserve, decimal in token units ("" restores the gateway's suggestion)
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

* [rail0 payouts destinations](rail0_payouts_destinations.md)	 - Manage where payouts go (lists when run bare)

