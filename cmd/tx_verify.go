package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
)

// signingContext carries what the pre-sign checks need, taken from the work
// ensureSignerFor already does: the payment it fetched and the address it derived
// from the signing key. Bundling them means verify costs no extra gateway call and
// no second key derivation. (#63)
//
// op and amount say what the transaction must DO, not just what it must act on: op is
// the operation ensureSignerFor checked the signer for, and pins the RAIL0 function
// selector; amount is the user's own --amount as typed, set by capture and refund —
// the two operations whose calldata carries an amount — and scaled by the payment's
// token decimals inside the SDK's check.
type signingContext struct {
	payment *rail0.Payment
	signer  string
	op      string
	amount  string
}

// verify checks a transaction the gateway asked the CLI to sign, BEFORE the user's
// key touches it. Until now the CLI signed whatever fields came back, so a hostile or
// merely misconfigured endpoint — reachable via --base-url or RAIL0_BASE_URL — could
// have the user's own key sign an arbitrary transaction.
//
// The checks are the SDK's (rail0.VerifyUnsignedTransaction, rail0-go v1.6.0 — a port
// of the copy that lived here): zero native value, the payment's chain, `from` = the
// signing key, `to` = the payment's RAIL0 contract, the selector of op (and, for
// capture and refund, the amount word = amount scaled by the token's decimals), the
// paymentId word = the payment's rail0_id. Every right-hand side is data already in
// hand, so this adds no network round-trip.
//
// `to` is compared with the payment's own rail0_contract. That is a consistency check,
// not an independent one — the address comes from the same gateway, and no public
// endpoint publishes the contract set on its own — so binding the calldata to this
// payment remains what closes the dangerous form of a swapped transaction.
//
// The SDK prefixes its refusals with the package name; the CLI's never did, so the
// prefix is dropped and the message is otherwise the SDK's — "refusing to sign: …",
// naming the field that disagrees.
func (sc *signingContext) verify(unsignedJSON string) error {
	err := rail0.VerifyUnsignedTransaction(unsignedJSON, sc.signer, rail0.TxExpectation{
		Payment:   sc.payment,
		Operation: sc.op,
		Amount:    sc.amount,
	})
	if err != nil {
		return errors.New(strings.TrimPrefix(err.Error(), "rail0: "))
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
//     response carries it: the only source is GET /blockchains (`contract.version`,
//     typed in the SDK as Blockchain.Contract.Version). Reading it would add a gateway
//     request to every create, which was judged not worth it for this check
//     (2026-09-24), so a nonce for different terms (another payee) is not detectable
//     here. Revisit if a payment response ever carries the version. `payments
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
