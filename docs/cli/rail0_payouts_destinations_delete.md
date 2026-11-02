## rail0 payouts destinations delete

Delete a destination (it leaves the lists; cannot be undone)

### Synopsis

Delete a payout destination. Only the destination's own wallet may.

It leaves the lists ('destinations --include-deleted' still shows it) and its
address can be registered again. The record is kept: its payouts and audit trail
keep naming it, and 'get' and 'events' still read it, with deleted_at set and
status "disabled". A deleted destination takes no further change and cannot be
re-enabled — to pause one instead, use 'disable'.

Its pending payout is settled first, as on 'disable': an unsigned one fails
"cancelled"; a SIGNED one refuses the delete (409 payout_already_signed), because
its transaction may still be mined — resend it or let it settle, then delete.

You are asked to confirm (y/N); pass --yes to skip the prompt in a script.

```
rail0 payouts destinations delete <destination_id> [flags]
```

### Examples

```
  rail0 payouts destinations delete <destination_id>
  rail0 payouts destinations delete <destination_id> --yes
```

### Options

```
  -h, --help   help for delete
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
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

