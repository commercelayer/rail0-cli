## rail0 auth login

Sign in with your Ethereum wallet (SIWE)

### Synopsis

Authenticates with the RAIL0 gateway using Sign-In with Ethereum (EIP-4361).

  1. Fetches a single-use nonce from the API
  2. Builds a SIWE message and signs it with your private key
  3. Verifies the signature and stores the JWT token

Any wallet can sign in — a gateway account is NOT required. Signing proves
control of the address, which is enough to read your own address-scoped
resources (e.g. 'payments list' shows every payment where you are payer or
payee). A gateway account only unlocks merchant-scoped features.

The token is saved to ~/.cache/rail0/token and used automatically by all
commands that require authentication (e.g. 'payments list') — that file is the
only place the session is read from. On success the command prints the session
(address, expiry) but NOT the raw token — pass --token (-t) to also print the
token itself, or run 'rail0 token' later.

The signing key is resolved like every other signing command: pass -p as a raw
0x-prefixed hex key, as @name for a key saved in the OS keychain ('rail0 keys
add'), or omit it to fall back to the RAIL0_PRIVATE_KEY environment variable.

```
rail0 auth login [flags]
```

### Examples

```
  rail0 auth login -p @payee     # or -p 0xYourPrivateKey…
  rail0 auth login -p @payee -t  # also print the token + its expiry
```

### Options

```
  -h, --help                 help for login
  -p, --private-key string   Signing key: 0x-prefixed hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY
  -t, --token                After signing in, also print the JWT token and its expiry to the terminal
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

* [rail0 auth](rail0_auth.md)	 - Authenticate with the RAIL0 gateway using SIWE

