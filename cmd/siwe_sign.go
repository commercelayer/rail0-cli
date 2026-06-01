package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// parseJWTClaims decodes the payload of a JWT without verifying the signature.
// Used only for display purposes (auth status).
func parseJWTClaims(jwt string) (map[string]any, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT")
	}
	b, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(b, &claims); err != nil {
		return nil, err
	}
	exp, ok := claims["exp"].(float64)
	if !ok || int64(exp) < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}
	return claims, nil
}

func decodeBase64URL(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	lookup := make(map[byte]int, 64)
	for i, c := range chars {
		lookup[byte(c)] = i
	}
	s = strings.TrimRight(s, "=")
	var bits, buf int
	var out []byte
	for i := 0; i < len(s); i++ {
		v, ok := lookup[s[i]]
		if !ok {
			return nil, fmt.Errorf("invalid base64 char")
		}
		buf = (buf << 6) | v
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(buf>>bits))
			buf &= (1 << bits) - 1
		}
	}
	return out, nil
}
