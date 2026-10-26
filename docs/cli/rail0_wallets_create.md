## rail0 wallets create

Register a new wallet on the account (currently unavailable)

### Synopsis

Register a wallet on the logged-in account.

CURRENTLY UNAVAILABLE: the gateway refuses this for every session (403
wallet_add_unavailable). A session is bound to the one wallet it signed in with;
adding another wallet returns with the merchant wallet-management section, which
keeps the proof described below. The command is kept for that.

The gateway requires a SIWE proof-of-ownership of the address being added: this
command fetches a single-use nonce, builds an EIP-4361 message for the added
address, signs it, and submits the message + signature. So you must hold the
private key OF THE ADDRESS you are adding, resolved exactly like the signing
commands — pass -p as a raw 0x hex key, as @name for an OS-keychain key
('rail0 keys add'), or omit it to fall back to RAIL0_PRIVATE_KEY (you are
prompted if none is set).

The message is NOT the one 'auth login' signs. The gateway pins each endpoint to
one statement — "Sign in to RAIL0" there, "Add this wallet to your RAIL0 account"
here — and refuses the other with 422 siwe_purpose_mismatch. That separation is
what stops a login signature, which you hand out on every sign-in, from being
replayed to bind an address to someone else's account.

The added address need NOT be your session address — a merchant may register
several payee wallets — but the key must match --address (the command derives
the signer from the key and rejects a mismatch). Addresses are globally unique:
one already registered to any account is refused (409).

```
rail0 wallets create [flags]
```

### Examples

```
  rail0 wallets create --address 0xABCD… -p @merchant-key --label "Main wallet"
  rail0 wallets create --address @merchant -p 0xYourKey…
```

### Options

```
  -A, --address string       EVM wallet address (0x-prefixed hex, or @alias)
  -h, --help                 help for create
  -l, --label string         Human-readable wallet label
  -p, --private-key string   Signing key OF the address being added: 0x hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY
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

* [rail0 wallets](rail0_wallets.md)	 - Manage the logged-in account's wallets (lists when run bare)

