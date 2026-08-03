package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// ansiRe matches the SGR colour escapes lipgloss emits, so a test can recover
// the plain text underneath.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// unquotedKeyRe matches an object key the colorized renderer emits without
// quotes (line-leading identifier followed by ": "), so a test can re-quote it
// and parse the output back as strict JSON to verify no field was dropped.
var unquotedKeyRe = regexp.MustCompile(`(?m)^(\s*)([A-Za-z0-9_]+): `)

func TestSortFields(t *testing.T) {
	in := []string{"updated_at", "status", "address", "active", "name", "id", "created_at", "label"}
	sortFields(in)
	// Identity/name first (priority order), then the rest alphabetically, with
	// timestamps (…_at) always trailing — note "status" comes before the dates
	// even though it sorts after "created_at" alphabetically.
	want := []string{"id", "name", "label", "address", "active", "status", "created_at", "updated_at"}
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("position %d: got %q, want %q (full: %v)", i, in[i], want[i], in)
		}
	}
}

func TestPrintPretty(t *testing.T) {
	cases := []string{
		`[{"chain_id":44787,"name":"Celo Sepolia","native_symbol":"CELO"}]`,
		`{"rail0_id":"0xabc123","status":"authorized","amount":"1000000"}`,
		`{"id":"019e","tokens":[{"symbol":"USDC","decimals":6}],"active":true}`,
	}
	for _, raw := range cases {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatal(err)
		}
		if err := printPretty(v); err != nil {
			t.Errorf("printPretty error: %v", err)
		}
	}
	// A nil result (e.g. a 204 delete) prints nothing and must not error.
	if err := printPretty(nil); err != nil {
		t.Errorf("printPretty(nil) error: %v", err)
	}
}

func TestPrintStyled(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{
			"chains (array of objects)",
			`[{"chain_id":44787,"name":"Celo Sepolia","native_symbol":"CELO","network_type":"testnet","explorer_url":"https://celo-alfajores.blockscout.com"},{"chain_id":5042002,"name":"Arc Testnet","native_symbol":"ETH","network_type":"testnet","explorer_url":""}]`,
		},
		{
			"tokens (array)",
			`[{"chain_id":5042002,"symbol":"USDC","address":"0x3600000000000000000000000000000000000000","decimals":6}]`,
		},
		{
			"payment (single object)",
			`{"rail0_id":"0xabc123","status":"authorized","amount":"1000000","payer":"0xBuyer","payee":"0xMerchant"}`,
		},
		{
			"transactions (array with status)",
			`[{"operation":"authorize","status":"confirmed","transaction_hash":"0xaaa"},{"operation":"capture","status":"submitted","transaction_hash":"0xbbb"}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			if err := json.Unmarshal([]byte(tc.raw), &v); err != nil {
				t.Fatal(err)
			}
			fmt.Printf("\n── %s ──\n", tc.name)
			if err := PrintStyled(v); err != nil {
				t.Errorf("PrintStyled error: %v", err)
			}

			// The colorized output uses unquoted keys, so it isn't strict JSON.
			// Strip the ANSI escapes, re-quote the keys, and it must decode back to
			// exactly the input — i.e. the renderer changes only colour, layout, and
			// key quoting, never the data (no field dropped).
			plain := ansiRe.ReplaceAllString(renderJSON(v, 0, ""), "")
			strict := unquotedKeyRe.ReplaceAllString(plain, `$1"$2": `)
			var got any
			if err := json.Unmarshal([]byte(strict), &got); err != nil {
				t.Fatalf("re-quoted output is not valid JSON: %v\n%s", err, strict)
			}
			if !reflect.DeepEqual(got, v) {
				t.Errorf("round-trip mismatch:\n got: %#v\nwant: %#v", got, v)
			}
		})
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// it printed. printQuiet's contract IS its stdout (scripts capture it with
// $(...)), so asserting the bytes is the only honest test.
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
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The scripting contract of --quiet: one id per line, on EVERY shape a command
// returns. The bare-list case is the regression pin for #101 — every list
// command returns resp.Data (a naked slice), and printQuiet only recognised
// the paginated envelope, so `payments list -q` dumped the full JSON.
func TestPrintQuietShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bare list (what every list command returns, #101)",
			`[{"rail0_id":"0xaaa"},{"rail0_id":"0xbbb"}]`, "0xaaa\n0xbbb\n"},
		{"paginated envelope",
			`{"data":[{"id":"one"},{"id":"two"}],"meta":{"total":2}}`, "one\ntwo\n"},
		{"single object",
			`{"rail0_id":"0xccc","status":"authorized"}`, "0xccc\n"},
		{"empty list prints nothing",
			`[]`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			if err := json.Unmarshal([]byte(tc.raw), &v); err != nil {
				t.Fatal(err)
			}
			got := captureStdout(t, func() {
				if err := printQuiet(v); err != nil {
					t.Errorf("printQuiet error: %v", err)
				}
			})
			if got != tc.want {
				t.Errorf("printQuiet output = %q, want %q", got, tc.want)
			}
		})
	}
}

// A non-empty list whose items carry no recognisable id (the chains catalog)
// must keep the JSON fallback rather than print nothing and hide the payload.
func TestPrintQuietIdlessListFallsBackToJSON(t *testing.T) {
	var v any
	if err := json.Unmarshal([]byte(`[{"chain_id":44787,"name":"Celo Sepolia"}]`), &v); err != nil {
		t.Fatal(err)
	}
	got := captureStdout(t, func() {
		if err := printQuiet(v); err != nil {
			t.Errorf("printQuiet error: %v", err)
		}
	})
	if !strings.Contains(got, "chain_id") {
		t.Errorf("expected the JSON fallback to carry the payload, got %q", got)
	}
}
