package cmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// signingContext carries what the pre-sign checks need, taken from the work
// ensureSignerFor already does: the payment it fetched and the address it derived
// from the signing key. Bundling them means verifyUnsignedTx costs no extra gateway
// call and no second key derivation. (#63)
//
// op and amountBaseUnits say what the transaction must DO, not just what it must act
// on: op is the operation ensureSignerFor checked the signer for, and pins the RAIL0
// function selector; amountBaseUnits is the user's own --amount scaled locally, set by
// capture and refund — the two operations whose calldata carries an amount.
type signingContext struct {
	payment         *rail0.Payment
	signer          string
	op              string
	amountBaseUnits string
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
// `to` is compared with the payment's own rail0_contract. That is a consistency check,
// not an independent one — the address comes from the same gateway, and no public
// endpoint publishes the contract set on its own — so binding the calldata to this
// payment (see checkCalldataPaymentID) remains what closes the dangerous form of a
// swapped transaction.
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

	// The call must go to THIS payment's RAIL0 contract. Only a consistency check —
	// the address comes from the same gateway as the transaction — but it costs
	// nothing and catches a response stitched together from two different payments.
	if sc.payment != nil && sc.payment.Rail0Contract != "" && tx.To != "" &&
		!strings.EqualFold(tx.To, string(sc.payment.Rail0Contract)) {
		return fmt.Errorf("refusing to sign: transaction is addressed to %s but this payment's RAIL0 contract is %s",
			tx.To, sc.payment.Rail0Contract)
	}

	// The call must be the RAIL0 function the user asked for. Checked before the
	// payment id so a swapped operation is named as such rather than as a wrong id.
	if sc.op != "" {
		if err := checkCalldataCall(tx.Data, sc.op, sc.amountBaseUnits); err != nil {
			return err
		}
	}

	if sc.payment != nil {
		return checkCalldataPaymentID(tx.Data, string(sc.payment.Rail0Id))
	}
	return nil
}

// rail0PaymentTuple is RAIL0.sol's `Payment` struct as the ABI encodes it: payer,
// payee, token, amount (uint120), authorizationExpiry and refundExpiry (uint48). The
// struct is all static types, so the tuple is encoded INLINE — six 32-byte words right
// after the paymentId — which is what fixes the position of the amount argument below.
const rail0PaymentTuple = "(address,address,address,uint120,uint48,uint48)"

// rail0FunctionSignatures are the eight RAIL0 entry points the CLI signs for, keyed by
// canonicalOp. Copied from RAIL0.sol (and identical to the gateway's ContractAbi,
// which records that no signature has changed since 1.1.0); TestRail0SelectorsMatchTheABI
// pins the resulting selectors against the contract's compiled methodIdentifiers.
var rail0FunctionSignatures = map[string]string{
	"authorize":     "authorize(bytes32," + rail0PaymentTuple + ",uint8,bytes32,bytes32)",
	"charge":        "charge(bytes32," + rail0PaymentTuple + ",uint8,bytes32,bytes32)",
	"capture":       "capture(bytes32," + rail0PaymentTuple + ",uint256)",
	"void":          "void(bytes32," + rail0PaymentTuple + ")",
	"release":       "release(bytes32," + rail0PaymentTuple + ")",
	"refund":        "refund(bytes32," + rail0PaymentTuple + ",uint256,uint8,bytes32,bytes32)",
	"dispute":       "dispute(bytes32," + rail0PaymentTuple + ",bytes32)",
	"close_dispute": "closeDispute(bytes32," + rail0PaymentTuple + ",bytes32)",
}

// rail0Selectors maps canonicalOp → the function's 4-byte selector as 8 lowercase hex
// chars: the first four bytes of keccak256(signature), computed once from the table
// above rather than hand-copied, so a signature and its selector cannot drift apart.
var rail0Selectors = func() map[string]string {
	out := make(map[string]string, len(rail0FunctionSignatures))
	for op, sig := range rail0FunctionSignatures {
		out[op] = hex.EncodeToString(ethcrypto.Keccak256([]byte(sig))[:4])
	}
	return out
}()

// rail0AmountWord is the calldata position (in 32-byte words after the selector) of
// the `uint256 amount` argument of capture and refund: word 0 is the paymentId, words
// 1-6 the inline Payment tuple, so the amount is word 7. Only these two operations
// take an amount; every other one moves a quantity the contract derives itself.
var rail0AmountWord = map[string]int{"capture": 7, "refund": 7}

// checkCalldataCall asserts the call is the RAIL0 function for op and, for capture and
// refund, that it moves exactly wantAmount base units.
//
// checkCalldataPaymentID alone bound the transaction to the right PAYMENT but not to
// the right OPERATION: every RAIL0 function takes the paymentId first, so a hostile
// endpoint could answer a void prepare with capture calldata (settling the escrow to
// the merchant instead of returning it), a capture prepare with release, or a dispute
// with closeDispute — each on the right payment, each passing that check. The
// selector closes that, and the amount word closes the remaining capture/refund swap:
// the right function for a different quantity than the user typed.
//
// Everything is FAIL-CLOSED: an operation with no known selector, or an amount-taking
// operation with no locally derived amount to compare, refuses rather than skipping.
// wantAmount is the user's input scaled by expectedBaseUnits — the gateway's word for
// the amount is never the right-hand side here.
func checkCalldataCall(data, op, wantAmount string) error {
	key := canonicalOp(op)
	want, ok := rail0Selectors[key]
	if !ok {
		return fmt.Errorf("refusing to sign: no known RAIL0 function for operation %q", op)
	}
	raw := strings.TrimPrefix(strings.ToLower(data), "0x")
	const selectorLen, wordLen = 8, 64 // hex chars
	if len(raw) < selectorLen {
		return fmt.Errorf("refusing to sign: calldata is %d hex chars, too short to carry a function selector", len(raw))
	}
	if got := raw[:selectorLen]; got != want {
		return fmt.Errorf("refusing to sign: the call is %s but you asked to %s (%s)",
			describeSelector(got), op, rail0FunctionSignatures[key])
	}

	word, takesAmount := rail0AmountWord[key]
	if !takesAmount {
		return nil
	}
	if wantAmount == "" {
		return fmt.Errorf("refusing to sign: cannot check the %s amount without the amount you asked for", op)
	}
	wantInt, ok := new(big.Int).SetString(wantAmount, 10)
	if !ok {
		return fmt.Errorf("refusing to sign: expected amount %q is not a base-unit integer", wantAmount)
	}
	start := selectorLen + word*wordLen
	if len(raw) < start+wordLen {
		return fmt.Errorf("refusing to sign: calldata is %d hex chars, too short to carry the %s amount", len(raw), op)
	}
	gotInt, ok := new(big.Int).SetString(raw[start:start+wordLen], 16)
	if !ok {
		return fmt.Errorf("refusing to sign: the %s amount in the calldata is not a number", op)
	}
	if gotInt.Cmp(wantInt) != 0 {
		return fmt.Errorf("refusing to sign: the call would %s %s base units but you asked for %s",
			op, gotInt, wantInt)
	}
	return nil
}

// describeSelector names a selector for an error message: the RAIL0 function it
// belongs to when it is one of ours (the likely shape of a swapped operation), and
// the bare hex otherwise.
func describeSelector(sel string) string {
	for op, s := range rail0Selectors {
		if s == sel {
			return rail0FunctionSignatures[op][:strings.IndexByte(rail0FunctionSignatures[op], '(')] + "()"
		}
	}
	return "selector 0x" + sel
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
// transaction to the payment the user actually asked to act on; checkCalldataCall
// binds it to the operation (the selector itself, and capture/refund's amount).
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
	// Recipient the authorization must pay: the payment's RAIL0 contract, taken
	// from the payment record. "" = unknown.
	to string
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
// What this does NOT establish, stated so nobody reads more into it:
//
//   - message.to, which for both RAIL0 flows is the RAIL0 contract, is compared only
//     with the payment record's rail0_contract — the same gateway's word, so not
//     independent (the limitation verify() documents). An authorization addressed to
//     a contract the endpoint controls is spendable by that contract for its value.
//   - message.nonce is NOT recomputed. For authorize/charge it is
//     keccak256(abi.encode(prefix, rail0_id, configHash)), and configHash is the
//     RAIL0-domain EIP-712 hash of the payment terms (payer, payee, token, amount,
//     both expiries) — so it is what binds the signature to THESE terms on-chain.
//     Recomputing it needs the deployment's VERSION() for the domain, and no payment
//     response carries it; GET /blockchains serves it as `contract.version`, but the
//     gateway's OpenAPI schema declares that property outside `properties`, so the
//     SDK's generated Blockchain type has no field for it. Until that is fixed a
//     nonce for different terms (another payee) is not detectable here. `payments
//     create` narrows this by checking the created payment's own terms against the
//     request (checkCreatedPayment), which catches a gateway that records the wrong
//     payment, but not one that records the right payment and serves a nonce for
//     another.
//
// The bounded residual is therefore an authorization for the RIGHT token, on the
// RIGHT chain, from the RIGHT signer, for EXACTLY the amount the user typed — a loss
// of at most the intended amount instead of an unbounded drain of the balance.
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

	// A consistency check against the payment record, like verify's `to` check.
	if want.to != "" && !strings.EqualFold(string(sp.Message.To), want.to) {
		return fmt.Errorf(
			"refusing to sign: the authorization pays %s but this payment's RAIL0 contract is %s",
			sp.Message.To, want.to)
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

// checkCreatedPayment asserts the payment `payments create` got back is the one it
// asked for, BEFORE the payer signs its authorization.
//
// verifySigningPayload holds the authorization to the request's signer, chain, token
// and amount, but the payment record itself went unchecked — and the record is what
// the payee then authorizes against: its payee is who the escrow settles to, its mode
// decides whether funds are held or forwarded at once. A gateway that created the
// payment with another payee, or as a charge when an authorize was asked for, had
// the payer sign for it. Every right-hand side here is the request the user built
// (flags or payment file), or the amount scaled locally from it, never the response.
//
// Addresses compare case-insensitively (the gateway may return checksummed ones);
// the amount compares in base units, exactly. An absent field in the response is a
// mismatch, not a skip: every one of these is always present on a created payment.
func checkCreatedPayment(created *rail0.Payment, req rail0.CreatePaymentRequest, wantUnits string) error {
	mismatch := func(field, got, want string) error {
		return fmt.Errorf("refusing to sign: the gateway created a payment with %s %q but you asked for %q", field, got, want)
	}
	switch {
	case created.ChainID != req.ChainID:
		return mismatch("chain_id", fmt.Sprint(created.ChainID), fmt.Sprint(req.ChainID))
	case !strings.EqualFold(string(created.Payer), string(req.Payer)):
		return mismatch("payer", string(created.Payer), string(req.Payer))
	case !strings.EqualFold(string(created.Payee), string(req.Payee)):
		return mismatch("payee", string(created.Payee), string(req.Payee))
	case !strings.EqualFold(string(created.Token), string(req.Token)):
		return mismatch("token", string(created.Token), string(req.Token))
	case created.Mode != req.Mode:
		return mismatch("mode", created.Mode, req.Mode)
	case created.Amount != wantUnits:
		return mismatch("amount (base units)", created.Amount, wantUnits)
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
		want.to = string(sc.payment.Rail0Contract)
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
// FAILS CLOSED. It used to return "" when the decimals could not be resolved (an
// unknown or retired token, a catalog read that failed) or the amount had more
// decimals than the token, and "" told verifySigningPayload to SKIP the value check —
// the one check the whole guard exists for. So a flaky catalog, or `-a 1.0000001` on
// USDC, had the authorization signed for whatever value the endpoint returned. The
// callers now refuse to sign instead.
func expectedBaseUnits(ctx context.Context, chainID int, token, humanAmount string) (string, error) {
	if humanAmount == "" || token == "" || chainID == 0 {
		return "", fmt.Errorf("refusing to sign: cannot check the authorization's amount without an amount, token and chain")
	}
	decimals, ok := tokenDecimals(ctx, chainID, token)
	if !ok {
		return "", fmt.Errorf("refusing to sign: the decimals of token %s on chain %d could not be resolved, "+
			"so the authorization's amount cannot be checked — see 'rail0 tokens --chain-id %d'", token, chainID, chainID)
	}
	units, err := rail0.ToBaseUnits(humanAmount, decimals)
	if err != nil {
		return "", fmt.Errorf("amount %q: %w", humanAmount, err)
	}
	return units, nil
}
