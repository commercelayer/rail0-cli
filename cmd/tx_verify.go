package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
)

// signingContext carries what the pre-sign checks need, taken from the work
// ensureSignerFor already does: the payment it fetched and the address it derived
// from the signing key. Bundling them means verifyUnsignedTx costs no extra gateway
// call and no second key derivation. (#63)
type signingContext struct {
	payment *rail0.Payment
	signer  string
}

// unsignedTx is the subset of the gateway's `unsigned_transaction` JSON that can be
// checked locally.
//
// Value stays as RAW JSON deliberately. A native quantity can legitimately arrive as
// a number or as a hex string, and a hostile one can be larger than any Go integer
// type — decoding into int64 or json.Number would turn those into a parse failure,
// which refuses the signature (safe) but reports "not valid JSON" instead of naming
// the actual problem. Keeping the raw bytes lets isZeroNativeValue judge every
// encoding and the error name the offending value.
type unsignedTx struct {
	ChainID int64           `json:"chain_id"`
	From    string          `json:"from"`
	To      string          `json:"to"`
	Value   json.RawMessage `json:"value"`
	Data    string          `json:"data"`
}

// verify checks a transaction the gateway asked the CLI to sign, BEFORE the user's
// key touches it. Until now the CLI signed whatever fields came back, so a hostile or
// merely misconfigured endpoint — reachable via --base-url or RAIL0_BASE_URL — could
// have the user's own key sign an arbitrary transaction.
//
// Every check here runs on data already in hand, so this adds no network round-trip.
//
// What it does NOT check, and why: the issue also asks that `to` equal the chain's
// RAIL0 contract. The CLI cannot do that — the contract address is exposed only on
// /sync/blockchains, which is HMAC-protected for the indexer; the public GET /chains
// returns Blockchain::Restricted, which has no contract address, and the catalog the
// CLI caches holds TOKENS, not contracts. Binding the calldata to this payment (see
// checkCalldataPaymentID) closes the dangerous form of that hole in the meantime.
func (sc *signingContext) verify(unsignedJSON string) error {
	var tx unsignedTx
	if err := json.Unmarshal([]byte(unsignedJSON), &tx); err != nil {
		return fmt.Errorf("refusing to sign: the gateway's unsigned transaction is not valid JSON: %w", err)
	}

	// Every RAIL0 operation is value-0: funds move as ERC-20 transfers driven by the
	// contract, never as native currency attached to the call. A non-zero value would
	// therefore be the gateway asking the user to hand over ETH.
	if !isZeroNativeValue(tx.Value) {
		return fmt.Errorf(
			"refusing to sign: transaction attaches native currency (value=%s); every RAIL0 operation is value-0",
			tx.Value)
	}

	if sc.payment != nil && sc.payment.ChainID != 0 && tx.ChainID != int64(sc.payment.ChainID) {
		return fmt.Errorf("refusing to sign: transaction targets chain %d but this payment is on chain %d",
			tx.ChainID, sc.payment.ChainID)
	}

	// The prepared `from` must be the key about to sign it. A mismatch means the
	// transaction was built for someone else's nonce and would be rejected on-chain
	// anyway — but it is also the shape a swapped-payload attack takes.
	if sc.signer != "" && tx.From != "" && !strings.EqualFold(tx.From, sc.signer) {
		return fmt.Errorf("refusing to sign: transaction is prepared from %s but the signing key is %s",
			tx.From, sc.signer)
	}

	if sc.payment != nil {
		return checkCalldataPaymentID(tx.Data, string(sc.payment.Rail0Id))
	}
	return nil
}

// isZeroNativeValue accepts the encodings a zero native value legitimately arrives
// in — absent, null, the number 0, or the string "0"/"0x0" — and treats everything
// else as non-zero.
//
// Deliberately a whitelist over the raw JSON: an encoding this does not recognise
// must refuse the signature rather than be guessed at, and reading the bytes directly
// means an oversized quantity is rejected on its merits instead of failing to parse.
func isZeroNativeValue(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	s = strings.TrimSpace(strings.Trim(s, `"`)) // a quantity sent as a JSON string
	return s == "" || s == "null" || s == "0" || strings.EqualFold(s, "0x0")
}

// checkCalldataPaymentID asserts the call's first argument is THIS payment's id.
//
// All eight RAIL0 operations take `bytes32 paymentId` as their first parameter
// (RAIL0.sol: authorize, charge, capture, void, release, refund, dispute,
// closeDispute), so under ABI encoding the id always occupies bytes 4..36 of the
// calldata — immediately after the 4-byte selector. Checking those bytes binds the
// transaction to the payment the user actually asked to act on, with no need for the
// contract ABI in the CLI.
//
// This is what stands in for the unavailable `to` check. A hostile gateway's most
// profitable swap is calldata for `approve(spender, MAX)` on the token contract,
// which this rejects: approve's first argument is a left-padded address, so its first
// 12 bytes are zero, whereas a rail0_id's leading 4 bytes are a sha256 checksum over
// the remainder and are not zero. More generally, any call whose first argument is
// not this payment's id is refused.
func checkCalldataPaymentID(data, rail0ID string) error {
	want := strings.TrimPrefix(strings.ToLower(rail0ID), "0x")
	if len(want) != 64 {
		// No usable rail0_id to compare against (e.g. a payment addressed by UUID
		// whose response omitted it). Nothing to assert rather than a false alarm.
		return nil
	}
	raw := strings.TrimPrefix(strings.ToLower(data), "0x")
	const selectorLen, argLen = 8, 64 // hex chars: 4-byte selector, 32-byte argument
	if len(raw) < selectorLen+argLen {
		return fmt.Errorf("refusing to sign: calldata is %d hex chars, too short to carry a paymentId", len(raw))
	}
	if got := raw[selectorLen : selectorLen+argLen]; got != want {
		return fmt.Errorf("refusing to sign: the call acts on payment 0x%s but you asked for 0x%s", got, want)
	}
	return nil
}

// ── EIP-712 / EIP-3009 authorization ─────────────────────────────────────────

// payloadExpectation is what the caller independently knows about the
// authorization it asked for. Every field is optional: a zero value means "not
// known here", and that check is skipped rather than guessed at.
type payloadExpectation struct {
	// Address derived from the private key about to sign. Always known.
	signer string
	// Chain the authorization must be scoped to. 0 = unknown.
	chainID int
	// Token contract the authorization must be against — the EIP-712 domain's
	// verifyingContract for EIP-3009. "" = unknown.
	token string
	// Exact amount in TOKEN BASE UNITS. "" = unknown (e.g. the token's decimals
	// could not be resolved, so the human amount cannot be scaled locally).
	valueBaseUnits string
}

// verifySigningPayload is signingContext.verify's counterpart for the EIP-712
// authorization, and exists for the same threat model: a hostile or merely
// misconfigured endpoint — reachable via --base-url or RAIL0_BASE_URL — could have
// the user's own key sign something they never asked for.
//
// It guarded only the EIP-1559 transaction, which is the LESS dangerous of the two
// artifacts. A malicious transaction costs gas and goes through RAIL0; an EIP-3009
// authorization is a bearer instrument the TOKEN honours directly, with no RAIL0
// involvement, for up to message.value — so an endpoint that answered a refund
// prepare with {from: <payee>, to: <attacker>, value: <the payee's whole balance>}
// got that signed verbatim and could replay it against USDC. Phase 2 and verify()
// never entered into it: the signature was already handed over.
//
// Asserting here does NOT break the sign-the-payload-verbatim rule (#93). The
// typehash still comes from the payload's own primaryType and the digest is still
// built from its message; this only refuses to sign a message whose fields
// contradict what the caller asked for. Every value compared is already in hand, so
// there is no extra round-trip — same property as verify().
//
// What it does NOT check, and why: message.to, which for both RAIL0 flows is the
// RAIL0 contract. The CLI cannot know that address — it is exposed only on
// /sync/blockchains, which is HMAC-protected for the indexer, and the catalog the
// CLI caches holds tokens, not contracts (the same limitation verify() documents).
// So the residual exposure is an authorization for the RIGHT token, on the RIGHT
// chain, from the RIGHT signer, for EXACTLY the amount the user typed, payable to an
// address of the endpoint's choosing. That is a bounded loss of the intended amount
// instead of an unbounded drain of the balance.
//
// One caveat worth stating: valueBaseUnits is derived by scaling the user's decimal
// amount with the token's decimals, and those decimals come from the catalog — i.e.
// from the same endpoint. A hostile one could overstate them to inflate the value it
// gets signed. The check still holds the amount to what the user typed under the
// endpoint's own declared scale, and every other field above is compared against
// something the user supplied or the CLI derived locally.
func verifySigningPayload(sp *rail0.SigningPayload, want payloadExpectation) error {
	if sp == nil {
		// Callers that require a payload already say so more precisely (create
		// answers 502 signing_payload_unavailable; sign reports the status).
		return nil
	}

	// The authorization must be FROM the key about to sign it. This is the one
	// comparison whose right-hand side is derived locally from the private key, so
	// it holds no matter what the endpoint claims.
	if want.signer != "" && !strings.EqualFold(string(sp.Message.From), want.signer) {
		return fmt.Errorf(
			"refusing to sign: the authorization is from %s but the signing key is %s",
			sp.Message.From, want.signer)
	}

	if want.chainID != 0 && sp.Domain.ChainId != want.chainID {
		return fmt.Errorf(
			"refusing to sign: the authorization is scoped to chain %d but this payment is on chain %d",
			sp.Domain.ChainId, want.chainID)
	}

	// For EIP-3009 the domain's verifyingContract IS the token: this is what stops an
	// authorization being minted against a different token the signer happens to hold.
	if want.token != "" && !strings.EqualFold(string(sp.Domain.VerifyingContract), want.token) {
		return fmt.Errorf(
			"refusing to sign: the authorization is against token %s but this payment's token is %s",
			sp.Domain.VerifyingContract, want.token)
	}

	// Exact, not a ceiling: the gateway signs for the full amount of the operation,
	// so anything else is a mismatch rather than a partial authorization.
	if want.valueBaseUnits != "" && string(sp.Message.Value) != want.valueBaseUnits {
		return fmt.Errorf(
			"refusing to sign: the authorization is for %s base units but you asked for %s",
			sp.Message.Value, want.valueBaseUnits)
	}

	return nil
}

// verifySigningPayload for a payment already fetched from the gateway: chain and
// token come from the payment record, the amount from its stored base-unit value.
func (sc *signingContext) verifySigningPayload(sp *rail0.SigningPayload, valueBaseUnits string) error {
	want := payloadExpectation{signer: sc.signer, valueBaseUnits: valueBaseUnits}
	if sc.payment != nil {
		want.chainID = sc.payment.ChainID
		want.token = string(sc.payment.Token)
	}
	return verifySigningPayload(sp, want)
}

// expectedBaseUnits scales the human amount the USER typed by the token's decimals,
// for comparison against the authorization's value.
//
// The user's own input is the trustworthy side here, which is why this scales that
// rather than reading the amount back off the gateway's response — comparing a
// gateway value against another gateway value would assert nothing.
//
// Returns "" when the decimals cannot be resolved (an unknown or retired token, or a
// catalog read that failed), so the caller skips the value check instead of comparing
// against a guessed scale.
func expectedBaseUnits(ctx context.Context, chainID int, token, humanAmount string) string {
	if humanAmount == "" || token == "" || chainID == 0 {
		return ""
	}
	decimals, ok := tokenDecimals(ctx, chainID, token)
	if !ok {
		return ""
	}
	units, err := rail0.ToBaseUnits(humanAmount, decimals)
	if err != nil {
		return ""
	}
	return units
}
