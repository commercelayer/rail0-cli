## rail0 payments dispute close

Close a dispute (payer only): prepare, sign, broadcast

```
rail0 payments dispute close <rail0Id> [flags]
```

### Examples

```
  rail0 payments dispute close 0xrail0Id… -p 0xPayerKey…
```

### Options

```
  -h, --help                 help for close
  -p, --private-key string   Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --reason string        bytes32 reason code (0x…); defaults to zero
      --timeout duration     Max time to wait when --wait is set (default 2m0s)
  -w, --wait                 Block until the operation confirms on-chain (or fails)
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

* [rail0 payments dispute](rail0_payments_dispute.md)	 - Open a dispute (payer-submitted, signal-only): prepare, sign, broadcast

