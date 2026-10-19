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
		return withoutSDKPrefix(err)
	}
	return nil
}

// ── EIP-712 / EIP-3009 authorization ─────────────────────────────────────────
//
// verify's counterpart for the EIP-712 authorization, for the same threat model: a
// hostile or merely misconfigured endpoint — reachable via --base-url or
// RAIL0_BASE_URL — could have the user's own key sign something they never asked for.
//
// The authorization is the MORE dangerous of the two artifacts. A malicious
// transaction costs gas and goes through RAIL0; an EIP-3009 authorization is a bearer
// instrument the TOKEN honours directly, with no RAIL0 involvement, for up to
// message.value — so an endpoint that answered a refund prepare with {from: <payee>,
// to: <attacker>, value: <the payee's whole balance>} would get that signed verbatim
// and could replay it against USDC.
//
// The check is the SDK's: every signing path goes through signSigningPayload
// (cmd/signing.go), which calls rail0.SignSigningPayloadFor (rail0-go v1.7.0, a port of
// the copy that lived here). It runs rail0.VerifySigningPayload BEFORE the key touches
// the payload — message.from = the signing key, domain.chainId, domain.verifyingContract
// = the token, message.to = the payment's RAIL0 contract, message.value exactly the
// expected base units — and signs only when it passes. Unlike the old local copy it
// FAILS CLOSED on every field: an expectation with anything missing (no rail0_contract,
// no chain, no amount) refuses instead of skipping that comparison.
//
// Asserting does NOT break the sign-the-payload-verbatim rule (#93): the typehash still
// comes from the payload's own primaryType and the digest from its message; this only
// refuses a message whose fields contradict what the caller asked for. Every value
// compared is already in hand, so there is no extra round-trip — same property as
// verify().
//
// What this does NOT establish, stated so nobody reads more into it:
//
//   - message.to is compared only with the payment record's rail0_contract — the same
//     gateway's word, so not independent (the limitation verify() documents).
//   - message.nonce is NOT recomputed. For authorize/charge it binds the signature to
//     the payment terms on-chain, but recomputing it needs the deployment's VERSION(),
//     which no payment response carries (only GET /blockchains does, and reading it
//     would add a request to every create — judged not worth it, 2026-09-24). `payments
//     create` narrows this with checkCreatedPayment, which catches a gateway that
//     records the wrong payment, but not one that records the right payment and serves
//     a nonce for another.
//
// The bounded residual is an authorization for the RIGHT token, on the RIGHT chain,
// from the RIGHT signer, for EXACTLY the amount asked — a loss of at most the intended
// amount instead of an unbounded drain of the balance.

// payloadExpectation is the expectation for an authorization over a payment the
// command already fetched (`payments sign`, `payments refund`): the SDK's
// rail0.PayloadExpectationFor — chain, token and RAIL0 contract from the payment
// record — with the signer this context derived from the key.
//
// amount "" holds the authorization to the payment's own stored base-unit amount (sign:
// the caller named a payment, not an amount, so the payment is what they agree to);
// otherwise amount is the human decimal the USER typed (refund's --amount), scaled by
// the payment's decimals. Fail-closed: unknown decimals or an amount finer than the
// token allows is an error, not a skipped check.
func (sc *signingContext) payloadExpectation(amount string) (rail0.PayloadExpectation, error) {
	want, err := rail0.PayloadExpectationFor(sc.payment, amount)
	if err != nil {
		return rail0.PayloadExpectation{}, withoutSDKPrefix(err)
	}
	want.Signer = rail0.Address(sc.signer)
	return want, nil
}

// withoutSDKPrefix drops the "rail0: " the SDK opens its errors with. The CLI's
// refusals never carried it, so the user keeps reading "refusing to sign: …" naming
// the field, in the SDK's own words (#174).
func withoutSDKPrefix(err error) error {
	return errors.New(strings.TrimPrefix(err.Error(), "rail0: "))
}

// checkCreatedPayment asserts the payment `payments create` got back is the one it
// asked for, BEFORE the payer signs its authorization.
//
// The SDK's payload check holds the authorization to the request's signer, chain, token
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

// expectedBaseUnits scales the human amount the USER typed by the token's decimals,
// for comparison against the authorization's value. Only `payments create` needs it:
// its expectation must come from the REQUEST, before any payment record exists, so the
// decimals are the catalog's; sign and refund scale against the fetched payment's own
// decimals through payloadExpectation.
//
// The user's own input is the trustworthy side here, which is why this scales that
// rather than reading the amount back off the gateway's response — comparing a
// gateway value against another gateway value would assert nothing.
//
// FAILS CLOSED. It used to return "" when the decimals could not be resolved (an
// unknown or retired token, a catalog read that failed) or the amount had more
// decimals than the token, and "" told the payload check to SKIP the value check —
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
