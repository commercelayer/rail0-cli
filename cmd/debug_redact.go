package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSON keys whose value must never reach the log, however deeply nested
// (CLAUDE.md rule 8). Matched case-insensitively against the marshalled body, so
// the rule follows the wire format rather than any one Go struct.
//
//   - signature          the SIWE login signature, and the payee's EIP-3009
//     authorization on a refund
//   - signed_transaction the RLP-encoded signed tx, which embeds that same
//     EIP-3009 authorization plus the caller's own ECDSA signature
//   - shared_secret      a webhook's HMAC secret, returned once by `webhooks add`
//     and `webhooks rotate-secret`
//   - token              the session JWT — see tokenLooksLikeJWT for why this one
//     is conditional
//
// `unsigned_transaction` and `signing_payload` are deliberately NOT here: they are
// what the user is about to sign, carry no secret, and are the single most useful
// thing to see when debugging a rejected operation.
var redactedLogKeys = map[string]bool{
	"signature":          true,
	"signed_transaction": true,
	"shared_secret":      true,
	"token":              true,
}

// redactBody prepares a request or response body for --debug logging.
//
// The SDK hands the logger the caller's typed struct (request) or the decoded
// response value, so redaction goes through JSON: marshal, walk the generic tree
// replacing sensitive values, and log that. Working on the marshalled shape means
// a new field on any SDK type is covered by its JSON key alone, with no list of Go
// types to keep in step.
//
// Values are replaced with their length rather than dropped, because "the
// signature was 132 characters" is exactly what makes a malformed-signature bug
// visible while the bytes themselves stay out of the terminal, the scrollback and
// any `2>` redirect. (#51)
func redactBody(body any) any {
	if body == nil {
		return nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		// Never fall back to logging the raw value: an unmarshallable body is
		// exactly the case where we cannot know what is in it.
		return "[body omitted: not serialisable]"
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "[body omitted: not serialisable]"
	}
	return redactValue(generic)
}

// redactValue walks a decoded JSON tree in place, replacing the values of
// redactedLogKeys. Recurses through objects and arrays so a secret nested inside a
// list (e.g. a paginated `data` array of webhooks) is caught too.
func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if redactedLogKeys[strings.ToLower(k)] && shouldRedact(k, val) {
				t[k] = redactedPlaceholder(val)
				continue
			}
			t[k] = redactValue(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = redactValue(val)
		}
		return t
	}
	return v
}

// shouldRedact resolves the one ambiguous key. `token` is a session JWT on an auth
// response, but on a payment it is the ERC-20 CONTRACT ADDRESS of the stablecoin —
// public, and one of the most useful fields to see while debugging a chain/token
// mismatch. Redacting both would trade a real secret for a real regression in
// debuggability, so the two are told apart by shape: a JWT is three dot-separated
// base64url segments, an address is 0x + 40 hex with no dots. Every other key in
// redactedLogKeys is unconditional.
func shouldRedact(key string, val any) bool {
	if strings.ToLower(key) != "token" {
		return true
	}
	s, ok := val.(string)
	return ok && strings.Count(s, ".") == 2
}

// redactedPlaceholder keeps the value's length (useful, non-sensitive) and drops
// its content.
func redactedPlaceholder(val any) string {
	if s, ok := val.(string); ok {
		return fmt.Sprintf("[redacted: %d chars]", len(s))
	}
	return "[redacted]"
}
