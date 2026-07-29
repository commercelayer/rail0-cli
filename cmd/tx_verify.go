package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	rail0 "github.com/rail0/go-sdk"
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
