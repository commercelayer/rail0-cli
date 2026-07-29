package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	rail0 "github.com/rail0/go-sdk"
)

// asJSON renders a redacted body the way the logger ultimately would, so a test can
// assert on the whole thing — including that a secret is absent from EVERY field,
// not just the one it was expected in.
func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal redacted body: %v", err)
	}
	return string(b)
}

// The three bodies that actually carry a secret over the wire, using the real SDK
// types the logger receives rather than hand-built maps.
func TestRedactBodyRemovesSecretsFromRealSDKTypes(t *testing.T) {
	const signedTx = "0x02f8b1018203e88459682f00850ba43b7400830249f094aaaa"
	const jwt = "header.payload.signature"
	const secret = "whsec_9f8e7d6c5b4a39281706"

	cases := []struct {
		name   string
		body   any
		secret string
	}{
		{
			name:   "submit request carries the signed transaction",
			body:   rail0.SubmitTransactionRequest{SignedTransaction: signedTx},
			secret: signedTx,
		},
		{
			name:   "auth response carries the session JWT",
			body:   &rail0.AuthResponse{Token: jwt, Address: "0xAbC", AccountID: "acc_1"},
			secret: jwt,
		},
		{
			name:   "webhook response carries the shared secret",
			body:   &rail0.Webhook{SharedSecret: secret},
			secret: secret,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// First prove the leak this fix closes is real: the body the SDK hands
			// the logger DOES contain the secret verbatim, which is what --debug
			// used to print. Without this the test below could pass against a body
			// that never carried the secret in the first place.
			if plain := asJSON(t, tc.body); !strings.Contains(plain, tc.secret) {
				t.Fatalf("test setup is wrong: %T does not carry the secret, so redaction proves nothing", tc.body)
			}
			got := asJSON(t, redactBody(tc.body))
			if strings.Contains(got, tc.secret) {
				t.Errorf("redactBody leaked the secret\n got: %s\n secret: %s", got, tc.secret)
			}
			if !strings.Contains(got, "[redacted") {
				t.Errorf("redactBody did not mark anything as redacted: %s", got)
			}
		})
	}
}

// The ambiguous key: `token` on a payment is the stablecoin CONTRACT ADDRESS and
// must survive, while `token` on an auth response is the JWT and must not.
func TestRedactBodyKeepsThePaymentTokenAddress(t *testing.T) {
	const usdc = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
	got := asJSON(t, redactBody(map[string]any{"token": usdc, "chain_id": 84532}))
	if !strings.Contains(got, usdc) {
		t.Errorf("redacted the token contract address, which is public: %s", got)
	}

	const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIweGFiYyJ9.c2ln"
	got = asJSON(t, redactBody(map[string]any{"token": jwt}))
	if strings.Contains(got, jwt) {
		t.Errorf("leaked a JWT under the token key: %s", got)
	}
}

// A secret nested inside a list — e.g. a paginated `data` array — is reached too.
func TestRedactBodyRecursesThroughArrays(t *testing.T) {
	const secret = "whsec_nested"
	got := asJSON(t, redactBody(map[string]any{
		"data": []any{
			map[string]any{"id": "wh_1", "shared_secret": secret},
		},
	}))
	if strings.Contains(got, secret) {
		t.Errorf("nested secret survived redaction: %s", got)
	}
	if !strings.Contains(got, "wh_1") {
		t.Errorf("redaction dropped a non-sensitive sibling field: %s", got)
	}
}

// Fields that are safe AND load-bearing for debugging must be preserved — the
// unsigned payload is the whole point of a --debug session on a rejected op.
func TestRedactBodyKeepsUnsignedPayloadAndIdentifiers(t *testing.T) {
	got := asJSON(t, redactBody(map[string]any{
		"unsigned_transaction": `{"to":"0xdead","chain_id":84532}`,
		"operation":            "capture",
		"status":               "pending",
	}))
	for _, want := range []string{"unsigned_transaction", "0xdead", "capture", "pending"} {
		if !strings.Contains(got, want) {
			t.Errorf("redaction removed %q, which is not sensitive: %s", want, got)
		}
	}
}

func TestRedactBodyHandlesNil(t *testing.T) {
	if got := redactBody(nil); got != nil {
		t.Errorf("redactBody(nil) = %v, want nil", got)
	}
}
