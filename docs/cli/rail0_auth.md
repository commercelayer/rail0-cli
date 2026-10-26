## rail0 auth

Authenticate with the RAIL0 gateway using SIWE

### Synopsis

Sign in to the gateway with an Ethereum wallet, using SIWE (EIP-4361).

Signing in proves you hold a key — it does not require a gateway account. ANY
wallet can log in; an account is what unlocks the merchant surface (wallets,
analytics, webhooks, disputes).

Who actually needs this:

  buyers      no. 'payments create' signs in on its own with the payer key,
              so paying needs nothing but a key.
  merchants   yes. The action commands (authorize, capture, refund, …) and
              every account view read the session.

The session is a JWT cached per gateway under the OS cache dir, so switching
'rail0 env' switches session with it rather than logging you out. The token is
never written to the config file. 'auth status' prints who you are and when it
expires; 'rail0 token' prints the token itself.

### Examples

```
  rail0 auth login -p @merchant-key
  rail0 auth status
  rail0 auth logout
```

### Options

```
  -h, --help   help for auth
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
* [rail0 auth login](rail0_auth_login.md)	 - Sign in with your Ethereum wallet (SIWE)
* [rail0 auth logout](rail0_auth_logout.md)	 - End the session: revoke the token on the gateway and remove it locally
* [rail0 auth revoke-all](rail0_auth_revoke-all.md)	 - End EVERY session of an address, on every machine (for a key you no longer trust)
* [rail0 auth status](rail0_auth_status.md)	 - Show the current authentication status

