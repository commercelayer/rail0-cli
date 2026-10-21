package cmd

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/spf13/cobra"
)

// humanizeError turns low-level transport and API errors into clear, actionable
// CLI messages. Anything it doesn't recognise (e.g. cobra usage errors) is
// returned unchanged.
//
// It knows nothing about the command that failed, so a 404 gets the neutral
// wording; Execute uses humanizeErrorFor, which does.
func humanizeError(err error) error {
	return humanizeErrorFor(nil, err)
}

// humanizeErrorFor is humanizeError for an error returned by cmd — the command
// cobra actually ran (nil when unknown). The command only matters for a 404, whose
// two possible causes differ by resource: see notFoundHint.
func humanizeErrorFor(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	// A give-up quotes the last transport error it saw; that text must not be
	// mistaken for the error itself. See ErrWaitGaveUp.
	if errors.Is(err, ErrWaitGaveUp) {
		return err
	}
	base := resolveBaseURL()

	// API responded with a non-2xx — map the common statuses to a hint.
	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == 400:
			// validation_failed and its siblings: the gateway's detail names the
			// offending field, so it goes through apiHint like every other status
			// rather than the SDK's bare "rail0 <code> (HTTP 400)" line.
			return fmt.Errorf("invalid request (HTTP 400)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 401:
			// The gateway distinguishes missing_authorization from token_expired and
			// invalid_token; say which one rather than collapsing all three.
			return fmt.Errorf("not authorized (HTTP 401)%s — run 'rail0 auth login' first", detail(apiHint(apiErr)))
		case apiErr.Status == 403:
			return fmt.Errorf("forbidden (HTTP 403)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 404:
			return fmt.Errorf("not found (HTTP 404)%s — %s", detail(apiHint(apiErr)), notFoundHint(cmd))
		case apiErr.Status == 409:
			return fmt.Errorf("already exists / conflict (HTTP 409)%s%s", detail(apiHint(apiErr)), conflictHint(cmd))
		case apiErr.Status == 422:
			return fmt.Errorf("request rejected (HTTP 422)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 429:
			return fmt.Errorf("rate limited (HTTP 429)%s", detail(rateLimitHint(apiErr)))
		case apiErr.Status == 501:
			return fmt.Errorf("the gateway cannot handle this request (HTTP 501)%s", detail(apiHint(apiErr)))
		case apiErr.Status >= 500:
			// Surface the gateway's own detail (e.g. signing_payload_unavailable /
			// rpc_unreachable) instead of masking every 5xx as "try again".
			return fmt.Errorf("the RAIL0 gateway reported an upstream error (HTTP %d)%s", apiErr.Status, detail(apiHint(apiErr)))
		default:
			return err // keep the SDK's message for anything else
		}
	}

	// No API response — a transport problem.
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr):
		return fmt.Errorf("can't resolve the API host for %s — check --base-url / RAIL0_BASE_URL", base)
	case errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(err.Error(), "connection refused"):
		return fmt.Errorf("can't reach the RAIL0 gateway at %s — is it running? (set --base-url or RAIL0_BASE_URL)", base)
	case isTimeout(err) || strings.Contains(err.Error(), "deadline exceeded"):
		return fmt.Errorf("the request to %s timed out", base)
	}
	return err
}

// notFoundHint says what a 404 can mean for the resource cmd works on.
//
// The gateway's scoped lookups (payments, webhooks, wallets, wallet tokens, event
// callbacks) answer 404, not 403, for something the session may not see, so
// "doesn't exist" and "isn't yours" are deliberately indistinguishable
// (gateway#157, #357) — the hint has to name both. (Another account's id is the
// exception: the gateway answers 403 not_your_account.) What "yours" means
// differs by resource, though: a payment is visible to its payer and payee only,
// so the likely fix is the signing key; a webhook, wallet, wallet token or event
// callback belongs to the session's account, where payer/payee advice
// (and -p, which those commands do not take) would send the user after the wrong
// thing. So the payer/payee wording is kept for commands under `rail0 payments`
// and every other command — or an unknown one — gets the neutral line.
func notFoundHint(cmd *cobra.Command) string {
	if isPaymentCommand(cmd) {
		return "the id may be wrong, or you are not the payer/payee of this payment (check the key you passed with -p)"
	}
	return "the id may be wrong, or it is not visible to this session ('rail0 auth status' shows who you are signed in as)"
}

// conflictHint is the command-specific tail of a 409, or "" when the gateway's own
// detail is all there is to say. Like a 404, a 409 is answered from a unique index
// rather than by the route, so its body is generic ("already exists") and cannot name
// the colliding field — only the command knows which fields could have collided.
func conflictHint(cmd *cobra.Command) string {
	if cmd == accountUpdateCmd {
		return " — " + accountConflictHint
	}
	return ""
}

// isPaymentCommand reports whether cmd is `rail0 payments` or one of its
// descendants (dispute, transaction, wait, …).
func isPaymentCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == paymentsCmd {
			return true
		}
	}
	return false
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// detail appends ": <msg>" when msg is non-empty, for tacking a server-provided
// reason onto a friendly prefix.
func detail(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return ""
	}
	return ": " + msg
}

// apiHint composes what the user reads after the HTTP status: the gateway's own
// sentence first, then this CLI's extra advice for that code.
//
// The gateway sends exactly code/title/detail, so `Detail` is a sentence written to be
// shown as-is and is always present — it is the text, not a fallback. The local hint is
// appended only when it says something the gateway cannot: which rail0 command to run
// next.
//
// The Message step is gone: the SDK now sets it FROM Detail (the gateway deleted the
// separate `message` rather than dual-sending it), so it could only ever repeat the
// value just found empty. Falling back to the bare Code stays — a body with no text at
// all is still better named by its code than by nothing.
func apiHint(e *rail0.APIError) string {
	msg := strings.TrimSpace(e.Detail)
	if msg == "" {
		msg = e.Code
	}
	if r := strings.TrimSpace(e.Reason); r != "" && !strings.Contains(msg, r) {
		msg = strings.TrimSpace(msg + " (" + r + ")")
	}
	if h := hintFor(e.Code, e.Reason); h != "" {
		msg = strings.TrimSpace(msg + " — " + h)
	}
	return msg
}

// rateLimitHint composes what follows "rate limited (HTTP 429)".
//
// The gateway shapes its own 429 through API::Errors and its detail already names the
// window — "Rate limit reached. Retry in 60 seconds." — so that sentence is preferred
// and the parsed Retry-After is deliberately NOT appended on top of it: the same
// number twice in one line reads like two different facts.
//
// The header is the fallback for a 429 the gateway did not produce. An upstream proxy,
// CDN or load balancer sends Retry-After with no rail0 error body, and that is exactly
// the case where the CLI has nothing else to tell the user. (#49)
func rateLimitHint(e *rail0.APIError) string {
	if msg := strings.TrimSpace(apiHint(e)); msg != "" {
		return msg
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("retry in %s", e.RetryAfter.Round(time.Second))
	}
	return "retry shortly"
}

// errorHints maps a gateway/contract error code to an actionable next step, so a
// rejected request explains what to do — mirroring the reasons the rail0-admin UI
// shows on a locked action. Keyed by the machine code (state guards and, for a
// contract_revert, the snake_case Solidity error).
var errorHints = map[string]string{
	// payment-state guards (HTTP 422)
	"amount_exceeds_capturable": "amount is above the capturable balance — see capturable_amount_human in 'rail0 payments get <id>'",
	"amount_exceeds_refundable": "amount is above the refundable balance — see refundable_amount_human in 'rail0 payments get <id>'",
	"not_capturable":            "the payment must be 'authorized' or 'partially_captured' to capture",
	"not_voidable":              "void is only allowed while 'authorized' or 'expired' with nothing captured — use 'release' for the remainder after a capture",
	"not_releasable":            "nothing uncaptured is left to release — the payment was already captured, voided or released ('rail0 payments get <id>' shows its status)",
	"not_refundable":            "nothing is refundable — the payment must be charged/captured and within the refund window",
	"not_signable":              "the payment must be 'unsigned' to sign",
	"already_signed":            "the payer signature is already stored — the payee can act now",
	"no_signature":              "the payer hasn't signed yet — run 'rail0 payments create' / 'payments sign' first",
	"wrong_mode":                "this operation doesn't match the payment's mode (authorize vs charge)",
	"already_disputed":          "a dispute is already open — close it with 'rail0 payments dispute close <id>' first",
	"not_disputed":              "there is no open dispute to close",
	"nothing_to_dispute":        "a dispute needs a merchant-held (refundable) balance",
	"unknown_dispute_reason":    "--reason must be a code from the right list (open to open a dispute, close to close one) — see 'rail0 disputes reasons'",
	"queue_unavailable":         "the gateway's job queue could not be reached, so nothing was enqueued — retry in a moment",
	"not_redrivable":            "only a pending transaction the gateway holds SIGNED can be redriven — check 'redrivable' in 'rail0 payments transaction <id> <transaction_id>'; an unsigned one needs the operation re-run",
	"signer_mismatch":           "the signing key doesn't match the payment's payer/payee — use the right wallet's key",
	// The SIWE BINDING failures, split out of signer_mismatch (gateway #216). These
	// matter more here than in a library: the CLI signs the GATEWAY's own host, so a
	// domain mismatch is a misconfigured base URL, and until this split it arrived
	// looking like a bad signature — sending the operator after their key instead of
	// their config. None of these echoes the server's allow-list; that endpoint is
	// unauthenticated, so naming the config would make the hint a probe. The chain-id hint
	// may point at GET /blockchains: the accepted ids (1 plus every active catalog chain,
	// gateway#426) are public there anyway.
	"siwe_domain_not_allowed":      "the gateway does not accept that origin — check `rail0 config` points at the right base URL, and have that host added to the gateway's SIWE domain allow-list",
	"siwe_uri_mismatch":            "the signed message's uri host must equal its own domain — this is a client bug, not a key problem",
	"siwe_chain_mismatch":          "sign with chain id 1 (what the CLI sends) or the id of a chain the gateway has active ('rail0 chains', GET /blockchains) — usually a hand-built message or a wallet on an unsupported or deactivated network",
	"siwe_proof_expired":           "the proof took too long between nonce and signature — run `rail0 auth login` again",
	"sessions_revoked":             "every session of this address from before its revoke-all cutoff is refused — run `rail0 auth login` again",
	"config_hash_mismatch":         "this payment and its on-chain deployment disagree — check it with 'rail0 payments get <id>'; it can't be operated as recorded",
	"payment_not_on_chain":         "the contract has no record of this payment — check its transactions with 'rail0 payments transactions <id>'",
	"unsupported_contract_version": "this payment's RAIL0 deployment isn't one the gateway supports — nothing to do client-side, the gateway needs upgrading",
	// token-level reverts: the token refused, not RAIL0
	"insufficient_token_balance":  "the paying wallet is short of the token — check it with 'rail0 wallets balances <id-or-address>'",
	"invalid_token_signature":     "the EIP-3009 signature didn't recover to the paying wallet — wrong key, chain, token or amount; re-run the command with the right -p key",
	"authorization_already_used":  "that authorization was already spent — each is single-use, create a fresh payment",
	"authorization_not_yet_valid": "the authorization isn't valid yet (validAfter is in the future)",
	"token_account_blocked":       "the token issuer has blocklisted one of the wallets in this transfer",
	"token_paused":                "the token contract is paused by its issuer — nothing can settle right now",
	// broadcast rejections: never reached the chain
	"insufficient_gas_funds":  "the sending wallet can't cover gas — fund it with the chain's native token ('rail0 chains' shows which)",
	"nonce_too_low":           "that nonce is already used on-chain — re-run the command to prepare a fresh transaction",
	"replacement_underpriced": "another transaction with that nonce is pending — wait for it with 'rail0 payments wait <id>'",
	"gas_price_too_low":       "the fee is below what the node accepts — re-run the command to pick up current fees",
	"already_known":           "the node already has this transaction — wait for it with 'rail0 payments wait <id>'",
	"rpc_unavailable":         "no RPC endpoint answered — the transaction was not submitted, retry in a moment",
	// authorization: which party the session is missing
	"not_the_payee": "only the merchant (payee) can do this — 'rail0 auth login' with the payee's key",
	"not_the_payer": "only the buyer (payer) can do this — 'rail0 auth login' with the payer's key",
	// deactivation and account scoping: the session is fine, the thing it acts for is not
	"account_deactivated": "the merchant account is deactivated — reads, void, release and refund still work; new payments, authorize, charge and capture need an operator to reactivate it",
	// A deactivated session wallet cannot write, with one exception (gateway#439): its
	// own session may switch it back on when its owner switched it off. Another wallet's
	// session cannot (gateway#411), and an operator's switch-off is the operator's to lift.
	"wallet_deactivated": "this wallet is deactivated and can only read — if you switched it off, switch it back on with 'rail0 wallets update <wallet> --active true' from its own session; otherwise sign in with another of the account's active wallets ('rail0 auth login')",
	// gateway#439: the owner's session reactivating a wallet an operator switched off (or
	// one switched off before the gateway recorded who did it). The switch-off revoked the
	// key, so that key's own session cannot lift it.
	"wallet_deactivated_by_operator": "an operator deactivated this wallet, so only an operator can reactivate it — ask an operator to reactivate it",
	"not_your_account":               "that account id is not the session's — pass your own to --account, or omit it to use the signed-in account ('rail0 auth status' shows which)",
	// A session is bound to the one wallet it signed in with (gateway#411): the account's
	// other wallets are readable, but update, delete and the token commands act only on
	// the session's own.
	"not_your_wallet": "this session can change only the wallet it signed in with — 'rail0 auth login' with that wallet's key to update it, deactivate it or manage its tokens ('rail0 auth status' shows which wallet you are signed in as)",
	// POST /accounts/:id/wallets is suspended for every session until the merchant
	// wallet-management section lands (gateway#411).
	"wallet_add_unavailable": "adding a wallet is not available yet — the gateway refuses 'rail0 wallets create' for every session until the merchant wallet-management section lands",
	// Login resolves the account FROM a wallet, so an account left with no active one
	// could neither act nor add a wallet back: the gateway refuses to switch off the last.
	// With adding suspended and the own-wallet rule, the way out is to switch another wallet
	// back on: from its own session if its owner switched it off (gateway#439), otherwise
	// through an operator.
	"last_active_wallet": "this is the account's only active wallet — another wallet must be active before this one can be switched off; adding one ('rail0 wallets create') is not available yet, so sign in with a wallet you switched off and run 'rail0 wallets update <wallet> --active true', or ask an operator to reactivate one",
	// Operator-side twin of last_active_wallet: the gateway keeps at least one active admin.
	"last_admin": "this would leave the gateway with no active operator — grant the operator role to another active account first, then retry",
	// POST /webhooks past the account's cap (WEBHOOKS_PER_ACCOUNT, 20 by default).
	// Disabled subscriptions count, deleted ones do not.
	"webhook_limit_reached": "the account already holds the most webhook subscriptions it may (disabled ones count too) — delete one ('rail0 webhooks list', 'rail0 webhooks delete <id>'), or add topics to an existing subscription with 'rail0 webhooks update'",
	// Not hinted: transaction_not_overwritable. The gateway raises it only on the
	// /…/submitted endpoints (reporting an externally broadcast hash onto a transaction
	// already confirmed), and the CLI never calls them — every operation here is
	// prepared, signed and submitted by the CLI itself.
	// `not_a_participant` was retired by gateway#157: a non-party now gets a plain
	// 404, deliberately, so a payment endpoint can't be used as an existence
	// oracle. The advice for that case therefore belongs on the 404 branch below,
	// not on a code the gateway no longer sends.
	// contract reverts (surfaced as contract_revert, or on a failed transaction)
	"not_payee":                  "only the merchant (payee) may do this — sign with the payee's key",
	"not_payer":                  "only the buyer (payer) may do this — sign with the payer's key",
	"not_payer_or_payee":         "only the payer or the payee may do this",
	"refund_expired":             "the refund window has closed — see refund_expiry_at in 'rail0 payments get <id>'; refund/dispute is no longer possible",
	"authorization_not_expired":  "release opens only after the authorization expires — authorization_expires_in in 'rail0 payments get <id>' says how long is left",
	"already_captured":           "already (partially) captured — use 'release' for the remainder, not 'void'",
	"unsupported_payment_method": "the payee (merchant) doesn't accept this token/chain — check 'rail0 payment-methods --address <payee>'",
	"idempotency_key_reused":     "that Idempotency-Key was already used for a payment with different terms — reuse it only to retry the same request, or pick a new key",
	"unknown_token":              "the token isn't configured on this chain — see 'rail0 tokens'",
	"unknown_chain":              "that chain id isn't configured on this gateway — see 'rail0 chains' for the ones it serves",
	"validation_failed":          "fix the field it names and retry — 'rail0 <command> --help' lists the accepted flags and values",
	// release is not permissionless: the contract accepts only the two parties as its
	// sender (gateway#356), and the CLI derives the sender from the signing key.
	"release_submitter_not_a_party": "only the payer or the payee can release — sign with one of their keys (-p)",
	"no_active_contract":            "no active RAIL0 contract on that chain — see 'rail0 chains'",
	"missing_param":                 "a required parameter is missing from the request",
	// A BARE forbidden is not a party mismatch: the gateway split those into codes of
	// their own (not_the_payee, not_the_payer, wallet_deactivated, account_deactivated,
	// not_your_account) because they need different fixes. This entry kept describing
	// one of them long after the split, and its remedy ('auth login' with the payer
	// key) was wrong for all three real causes — the rule it named no longer exists in
	// the gateway at all.
	"forbidden":              "not permitted for this session — typically the operator grant, a resource owned by another account, or a transaction signed by the wrong wallet ('rail0 auth status' shows who you are signed in as)",
	"payment_already_exists": "a payment with this id already exists on-chain",
}

// hintFor returns the actionable hint for an error code (preferred) or its
// reason sub-cause (used when a contract_revert carries the specific error there).
func hintFor(code, reason string) string {
	if h := errorHints[code]; h != "" {
		return h
	}
	return errorHints[reason]
}

// ErrAborted marks an operation the USER cancelled — Ctrl-C or ESC at a prompt, or
// declining a destructive confirmation. Distinct from a failure: nothing went wrong,
// the answer was "no".
//
// It still must not exit 0. `rail0 payments refund <id> && mark_refunded_in_db` used
// to run the second command when the operator hit Ctrl-C at the amount prompt, so a
// database was marked refunded with no refund behind it. (#65)
var ErrAborted = errors.New("aborted")

// ExitCodeAborted is the status for a user cancellation. 130 is the POSIX convention
// for a process terminated by SIGINT (128 + 2) — what a shell reports for Ctrl-C — so
// scripts and CI already recognise it, and it cannot collide with the API-derived
// codes below.
const ExitCodeAborted = 130

// ExitCodeRateLimited is the status for an HTTP 429. Distinct so a script can tell
// "slow down and retry" apart from a real failure — the one API error class where
// re-running the identical command later is the correct response. (#49)
const ExitCodeRateLimited = 10

// exitCode maps an error to a distinct process exit status so scripts can branch
// on the class of failure: 3 auth, 4 forbidden, 5 not-found, 6 conflict,
// 7 rejected (422), 8 upstream (5xx), 9 network, 10 rate-limited,
// 130 user-cancelled, 1 otherwise (including a --wait that gave up).
func exitCode(err error) int {
	// Checked before the API/transport classification: a cancellation is not a
	// failure of the request, and may not even have made one.
	if errors.Is(err, ErrAborted) {
		return ExitCodeAborted
	}
	// Not a network failure, whatever read error the message quotes: the operation
	// may be in flight, so this must never read as "safe to retry" (9).
	if errors.Is(err, ErrWaitGaveUp) {
		return 1
	}

	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == 0:
			return 9 // no HTTP response — a network-level failure
		case apiErr.Status == 401:
			return 3
		case apiErr.Status == 403:
			return 4
		case apiErr.Status == 404:
			return 5
		case apiErr.Status == 409:
			return 6
		case apiErr.Status == 422:
			return 7
		case apiErr.Status == 429:
			return ExitCodeRateLimited
		case apiErr.Status >= 500:
			return 8
		}
	}
	// Transport-level failure (no usable HTTP response). Mirror humanizeError's
	// detection, including the string fallbacks — the SDK wraps some network
	// errors so they no longer unwrap to net.Error / ECONNREFUSED.
	var dnsErr *net.DNSError
	var ne net.Error
	s := err.Error()
	if errors.As(err, &dnsErr) || errors.As(err, &ne) || errors.Is(err, syscall.ECONNREFUSED) ||
		isTimeout(err) ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "deadline exceeded") {
		return 9
	}
	return 1
}
