package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// parseJWTClaims decodes the payload of a JWT WITHOUT verifying its signature.
//
// NOT display-only, despite what this comment used to claim: ensureSession calls it
// to decide whether the cached session can be reused (cmd/root.go), so an error here
// is what sends a command through a fresh SIWE login. Verifying the signature is the
// gateway's job — this only reads the claims the CLI needs (`sub`, `account_id`,
// `exp`).
//
// A live `exp` is part of the contract: callers treat a nil error as "these claims
// are usable", and tokenExpiry asserts claims["exp"].(float64) unchecked on that
// basis. The failure modes are now reported distinctly instead of all arriving as
// "token expired". No caller surfaces the text today — each flattens it into its own
// "invalid or expired" hint — so that part is for whoever next needs it. (#71)
func parseJWTClaims(jwt string) (map[string]any, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed token: expected 3 dot-separated JWT segments, got %d", len(parts))
	}
	// A JWT's segments are base64url with the padding stripped — precisely what
	// base64.RawURLEncoding decodes. The hand-rolled decoder this replaces
	// substituted the URL-safe characters by hand, appended `=` padding and then
	// trimmed that same padding straight back off (so the padding block was dead),
	// and rebuilt a 64-entry lookup map on every call.
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed token: payload is not base64url: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("malformed token: payload is not JSON: %w", err)
	}
	// json.Unmarshal decodes every JSON number as float64, so exp arrives as one.
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, fmt.Errorf("malformed token: missing or non-numeric exp claim")
	}
	if int64(exp) < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}
	return claims, nil
}
