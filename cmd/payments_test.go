package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/zalando/go-keyring"
)

// allowedSigners must mirror the gateway's ensure_signed_by! guard: payee-only
// for the fund operations, payer-only for disputes, payer-or-payee for release.
func TestAllowedSigners(t *testing.T) {
	p := &rail0.Payment{Payer: "0xPayer", Payee: "0xPayee"}
	cases := []struct {
		op   string
		want []string
	}{
		{"authorize", []string{"0xPayee"}},
		{"charge", []string{"0xPayee"}},
		{"capture", []string{"0xPayee"}},
		{"void", []string{"0xPayee"}},
		{"refund", []string{"0xPayee"}},
		{"release", []string{"0xPayer", "0xPayee"}},
		{"dispute", []string{"0xPayer"}},
		{"close dispute", []string{"0xPayer"}},
		// The wire spelling must reach the same rule as the label. Before #72 this
		// fell through to the payee-only default and rejected the payer, who is the
		// only party allowed to close a dispute.
		{"close_dispute", []string{"0xPayer"}},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			got, err := allowedSigners(tc.op, p)
			if err != nil {
				t.Fatalf("allowedSigners(%q) returned an error: %v", tc.op, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("allowedSigners(%q) = %v, want %v", tc.op, got, tc.want)
			}
		})
	}
}

// An operation with no signer rule must fail loudly. The previous `default:` branch
// answered payee-only, which is the exact opposite of correct for the payer-only
// operations and surfaced as a misleading "wrong key?" error.
func TestAllowedSignersRejectsAnUnknownOperation(t *testing.T) {
	p := &rail0.Payment{Payer: "0xPayer", Payee: "0xPayee"}
	for _, op := range []string{"", "settle", "close-dispute", "captur"} {
		got, err := allowedSigners(op, p)
		if err == nil {
			t.Errorf("allowedSigners(%q) = %v with no error; want an error rather than a silent default", op, got)
		}
		if got != nil {
			t.Errorf("allowedSigners(%q) returned %v alongside an error; want nil", op, got)
		}
	}
}

func TestResolveSigningKey(t *testing.T) {
	t.Setenv("RAIL0_PRIVATE_KEY", "")

	// Explicit key wins.
	if got, err := resolveSigningKey("0xexplicit"); err != nil || got != "0xexplicit" {
		t.Errorf("explicit: got %q err %v", got, err)
	}
	// No key anywhere → error.
	if _, err := resolveSigningKey(""); err == nil {
		t.Error("no key: expected error, got nil")
	}
	// RAIL0_PRIVATE_KEY fallback (CI).
	t.Setenv("RAIL0_PRIVATE_KEY", "0xenvkey")
	if got, err := resolveSigningKey(""); err != nil || got != "0xenvkey" {
		t.Errorf("env fallback: got %q err %v", got, err)
	}
}

// signingKeyOrPrompt must pass a resolvable key straight through WITHOUT
// prompting — so a key from --private-key or RAIL0_PRIVATE_KEY never triggers the
// hidden prompt. (The no-key path prompts/reads stdin and is not exercised here,
// since that would block on an interactive terminal.)
func TestSigningKeyOrPrompt_PassThrough(t *testing.T) {
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	if got, err := signingKeyOrPrompt("0xexplicit"); err != nil || got != "0xexplicit" {
		t.Errorf("explicit: got %q err %v", got, err)
	}
	t.Setenv("RAIL0_PRIVATE_KEY", "0xenvkey")
	if got, err := signingKeyOrPrompt(""); err != nil || got != "0xenvkey" {
		t.Errorf("env fallback: got %q err %v", got, err)
	}
}

// An "@name" typed at the interactive prompt must resolve to the saved keychain
// key, exactly like the --private-key flag.
func TestSigningKeyOrPrompt_PromptAlias(t *testing.T) {
	keyring.MockInit() // in-memory keychain — never touches the real OS store
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	if err := keyring.Set(keyringService, "payer", testKeyHex); err != nil {
		t.Fatalf("seed keychain: %v", err)
	}
	// Feed "@payer" to the hidden prompt via piped stdin (the non-TTY read path).
	old := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() { io.WriteString(w, "@payer\n"); w.Close() }()

	if got, err := signingKeyOrPrompt(""); err != nil || got != testKeyHex {
		t.Fatalf("prompt @name: got %q err %v", got, err)
	}
}

// A key that was SUPPLIED but does not resolve is an error, never a cue to prompt:
// with a piped stdin the prompt used to take the next line as the private key, and
// the "unknown key" that explained the failure was thrown away.
func TestSigningKeyOrPrompt_UnknownKeyErrorsInsteadOfPrompting(t *testing.T) {
	keyring.MockInit()
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	old := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	go func() { io.WriteString(w, testKeyHex+"\n"); w.Close() }()

	got, err := signingKeyOrPrompt("@doesnotexist")
	if err == nil {
		t.Fatalf("an unknown @name must fail, got key %q (read from stdin)", got)
	}
	if !strings.Contains(err.Error(), "doesnotexist") {
		t.Errorf("error must name the key that was not found, got %v", err)
	}
}

// The atomic operations no longer expose --prepare: it should be absent from
// every signing command so the help can't promise a flow the CLI can't complete.
func TestSigningCommandsHaveNoPrepareFlag(t *testing.T) {
	cmds := []string{"authorize", "charge", "capture", "void", "release", "refund", "dispute"}
	for _, name := range cmds {
		sub, _, err := paymentsCmd.Find([]string{name})
		if err != nil || sub == nil {
			t.Fatalf("payments %s: command not found (%v)", name, err)
		}
		if sub.Flags().Lookup("prepare") != nil {
			t.Errorf("payments %s: --prepare should have been removed", name)
		}
		if sub.Flags().Lookup("private-key") == nil {
			t.Errorf("payments %s: --private-key flag missing", name)
		}
	}
}

// One page of `payments list` carries the same human companions as --all and
// `payments get`: a flag that only changes how many rows are fetched must not change
// the shape of each row.
func TestPaymentsListHumanizesASinglePage(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)
	seedDecimals(t, 5042002, testToken, 6)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": "p-1", "rail0_id": "0xabc", "status": "authorized",
			"chain_id": 5042002, "token": testToken, "amount": "4050000",
		}})
	}))
	defer srv.Close()
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	out := captureStdout(t, func() {
		if err := paymentsListCmd.RunE(paymentsListCmd, nil); err != nil {
			t.Errorf("payments list: %v", err)
		}
	})
	if !strings.Contains(out, `"amount_human": "4.05"`) {
		t.Errorf("a single page must carry amount_human, got:\n%s", out)
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
