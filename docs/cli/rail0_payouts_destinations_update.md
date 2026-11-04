## rail0 payouts destinations update

Change a destination's label or minimum

### Synopsis

Change a payout destination's label or minimum deposit. Only the destination's
own wallet may.

--label is trimmed and may not be blank (label_required, refused before any
request); it must stay unique in the account, ignoring case (409
payout_destination_label_taken when another destination already has it).

--min-amount is a decimal in token units, converted with the destination token's
decimals. The minimum cannot be blanked: --min-amount "" is refused — to state that the venue has
no minimum, pass --no-minimum. --min-amount and --no-minimum exclude each other. A
destination registered before the minimum was asked shows it as "not set": state it
here.

CHANGING the minimum (a new figure, or --no-minimum on one that had a figure)
asks for the details confirmation again, exactly like 'add' (a y/N prompt on a
terminal, or --confirm-details; --yes does not count): the minimum is copied from
the venue, and a wrong one is how a deposit gets lost.
Re-sending the current value asks nothing. Raising it above the destination's
latest proof takes the verification away: the destination goes back to
"unverified" and needs a new proof at the new minimum.

To switch a destination off or on, use 'disable' / 'enable'; to remove it, 'delete'.
The reserves a payout keeps back are decided at payout time ('payouts send').

```
rail0 payouts destinations update <destination_id> [flags]
```

### Examples

```
  rail0 payouts destinations update <destination_id> --label "Exchange (main)"
  rail0 payouts destinations update <destination_id> --min-amount 25 --confirm-details
  rail0 payouts destinations update <destination_id> --no-minimum --confirm-details
```

### Options

```
      --confirm-details     Accept the details statement without a prompt (needed without a TTY or with --agent); --yes does NOT imply it
  -h, --help                help for update
  -l, --label string        New label — not blank, unique in the account (ignoring case)
      --min-amount string   New minimum deposit, decimal in token units; asks for the details confirmation
      --no-minimum          The venue has no minimum deposit (no_minimum: true; removes a figure); asks for the details confirmation
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

