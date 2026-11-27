package cmd

import (
	"net/http"
	"strings"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
)

func TestAdminDisputesSendsItsFilters(t *testing.T) {
	g := newOperatorGateway(t)
	rail0ID := "0x" + strings.Repeat("ab", 32)
	setFlags(t, adminDisputesCmd, map[string]string{
		"status": "open", "payment": rail0ID, "reason": "not_received",
		"from": "2026-11-01T00:00:00Z", "to": "2026-11-27T00:00:00Z", "sort": "-opened_at",
	})
	adminDispChainID = 5042002
	t.Cleanup(func() { adminDispChainID = 0 })
	if _, err := runE(t, adminDisputesCmd); err != nil {
		t.Fatalf("disputes: %v", err)
	}
	q := g.last(t, http.MethodGet, "/admin/disputes").query
	for key, want := range map[string]string{
		"status": "open", "payment_id": rail0ID, "chain_id": "5042002", "reason_code": "not_received",
		"from": "2026-11-01T00:00:00Z", "to": "2026-11-27T00:00:00Z", "sort": "-opened_at",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}

	for flag, bad := range map[string]string{"status": "pending", "reason": "no_such_reason", "payment": "0x12"} {
		before := len(g.requests())
		setFlags(t, adminDisputesCmd, map[string]string{"status": "", "reason": "", "payment": ""})
		setFlags(t, adminDisputesCmd, map[string]string{flag: bad})
		if _, err := runE(t, adminDisputesCmd); err == nil || exitCode(err) != ExitCodeUsage || len(g.requests()) != before {
			t.Errorf("--%s %s: err = %v, %d requests sent", flag, bad, err, len(g.requests())-before)
		}
	}
}

// operatorsGateway answers the role's routes and the account read the two writes make.
func operatorsGateway(t *testing.T) *operatorGateway {
	t.Helper()
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		switch {
		case r.URL.Path == "/admin/administrators":
			return http.StatusOK, []any{
				map[string]any{"id": "g-2", "account_id": "acc-9", "granted_by": "acc-1", "note": "on call",
					"expires_at": "2020-01-01T00:00:00Z", "account": map[string]any{"name": "Bea"}, "grantor": map[string]any{"name": "Ada"}},
				map[string]any{"id": "g-1", "account_id": "acc-1", "account": map[string]any{"name": "Ada"}},
			}
		case r.URL.Path == "/accounts/acc-9":
			return http.StatusOK, map[string]any{"id": "acc-9", "name": "Bea", "email": "bea@example.com", "active": true}
		case r.URL.Path == "/admin/administrators/acc-9" && r.Method == http.MethodPost:
			return http.StatusCreated, map[string]any{"id": "g-3", "account_id": "acc-9", "granted_by": "acc-1"}
		case r.URL.Path == "/admin/administrators/acc-9" && r.Method == http.MethodDelete:
			return http.StatusNoContent, nil
		}
		return 0, nil
	}
	return g
}

func TestAdminOperatorsList(t *testing.T) {
	operatorsGateway(t)
	out, err := runE(t, adminOperatorsCmd)
	if err != nil || !strings.Contains(out, `"account_id": "acc-9"`) || !strings.Contains(out, `"note": "on call"`) {
		t.Fatalf("operators: %v\n%s", err, out)
	}
	expired := map[string]any{"expires_at": "2020-01-01T00:00:00Z"}
	if got := grantExpiryCell(expired); !strings.HasSuffix(got, "(expired)") {
		t.Errorf("an expired grant reads %q", got)
	}
	ahead := map[string]any{"expires_at": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)}
	if got := grantExpiryCell(ahead); got == "" || strings.Contains(got, "expired") {
		t.Errorf("a grant still ahead reads %q", got)
	}
	if got := grantExpiryCell(map[string]any{}); got != "" {
		t.Errorf("no expiry reads %q", got)
	}
}

// Granting the role: write mode first, then the account is read and named in a question,
// and only a yes sends the grant, with the note and the expiry worked out from --expires.
func TestAdminOperatorsGrant(t *testing.T) {
	g := operatorsGateway(t)
	setFlags(t, adminOperatorsGrantCmd, map[string]string{"note": " on call this week ", "expires": "7d"})

	if _, err := runE(t, adminOperatorsGrantCmd, "acc-9"); agentErrorCode(err, exitCode(err)) != "write_mode_required" {
		t.Fatalf("without write mode: err = %v", err)
	}
	if n := len(g.requests()); n != 0 {
		t.Fatalf("%d requests were sent without write mode", n)
	}

	wt := writeModeOn(t)
	if _, err := runE(t, adminOperatorsGrantCmd, "acc-9"); agentErrorCode(err, exitCode(err)) != "confirmation_required" {
		t.Fatalf("without --yes: err = %v", err)
	}
	if !g.sent(http.MethodGet, "/accounts/acc-9") || g.sent(http.MethodPost, "/admin/administrators/acc-9") {
		t.Fatal("the account must be read before the question, and the role not granted without an answer")
	}

	answerYes(t)
	before := time.Now()
	out, err := runE(t, adminOperatorsGrantCmd, "acc-9")
	if err != nil || !strings.Contains(out, `"granted_by": "acc-1"`) {
		t.Fatalf("grant: %v\n%s", err, out)
	}
	req := g.last(t, http.MethodPost, "/admin/administrators/acc-9")
	body := req.json(t)
	if req.token != wt || body["note"] != "on call this week" {
		t.Errorf("token %q, body %v", req.token, body)
	}
	at, perr := time.Parse(time.RFC3339, body["expires_at"].(string))
	if want := before.Add(7 * 24 * time.Hour); perr != nil || at.Before(want.Add(-time.Minute)) || at.After(want.Add(time.Minute)) {
		t.Errorf("expires_at = %v (%v), want about %v", body["expires_at"], perr, want.UTC())
	}

	// No note and no expiry: neither is sent.
	setFlags(t, adminOperatorsGrantCmd, map[string]string{"note": "", "expires": ""})
	if _, err := runE(t, adminOperatorsGrantCmd, "acc-9"); err != nil {
		t.Fatalf("a bare grant: %v", err)
	}
	if body := g.last(t, http.MethodPost, "/admin/administrators/acc-9").json(t); len(body) != 0 {
		t.Errorf("a bare grant sent %v", body)
	}
}

func TestAdminOperatorsRevoke(t *testing.T) {
	g := operatorsGateway(t)
	if _, err := runE(t, adminOperatorsRevokeCmd, "acc-9"); agentErrorCode(err, exitCode(err)) != "write_mode_required" {
		t.Fatalf("without write mode: err = %v", err)
	}
	wt := writeModeOn(t)
	if _, err := runE(t, adminOperatorsRevokeCmd, "acc-9"); agentErrorCode(err, exitCode(err)) != "confirmation_required" {
		t.Fatalf("without --yes: err = %v", err)
	}
	if g.sent(http.MethodDelete, "/admin/administrators/acc-9") {
		t.Fatal("the role was revoked without a confirmation")
	}
	answerYes(t)
	out, err := runE(t, adminOperatorsRevokeCmd, "acc-9")
	if err != nil || !strings.Contains(out, `"revoked": true`) || !strings.Contains(out, `"account_id": "acc-9"`) {
		t.Fatalf("revoke: %v\n%s", err, out)
	}
	if req := g.last(t, http.MethodDelete, "/admin/administrators/acc-9"); req.token != wt {
		t.Errorf("the revoke presented %q, want the write-mode token", req.token)
	}
}

func TestParseGrantExpiry(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]string{
		"":                          "",
		"7d":                        "2026-12-03T12:00:00Z",
		"12h":                       "2026-11-27T00:00:00Z",
		"1h30m":                     "2026-11-26T13:30:00Z",
		"2026-12-31T23:59:59Z":      "2026-12-31T23:59:59Z",
		"2026-12-31T23:59:59+01:00": "2026-12-31T22:59:59Z",
	} {
		got, err := parseGrantExpiry(in, now)
		if err != nil || got != want {
			t.Errorf("parseGrantExpiry(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"0d", "-3d", "soon", "2026-11-26", "2026-11-26T11:00:00Z", "-2h"} {
		if got, err := parseGrantExpiry(in, now); err == nil || exitCode(err) != ExitCodeUsage {
			t.Errorf("parseGrantExpiry(%q) = %q, %v; want a usage error", in, got, err)
		}
	}
}

// The question before a grant names the account and says what the role opens.
func TestGrantOperatorPrompt(t *testing.T) {
	a := &rail0.AccountFull{ID: "acc-9", Name: "Bea", Email: "bea@example.com", Active: true}
	got := grantOperatorPrompt(a, "")
	for _, want := range []string{`Grant the operator role to account "Bea" (acc-9)?`, "bea@example.com", "read every account", "no expiry"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt %q does not say %q", got, want)
		}
	}
	a.Active = false
	got = grantOperatorPrompt(a, "2026-12-03T12:00:00Z")
	if !strings.Contains(got, "DEACTIVATED") || !strings.Contains(got, "until 2026-12-03") || strings.Contains(got, "no expiry") {
		t.Errorf("prompt for a deactivated account with an expiry = %q", got)
	}
}

// A duplicate grant is answered by a unique index, whose 409 cannot say what collided:
// the command says it.
func TestOperatorGrantConflictHint(t *testing.T) {
	if got := conflictHint(adminOperatorsGrantCmd); !strings.Contains(got, "already holds an operator grant") {
		t.Errorf("hint = %q", got)
	}
	if got := conflictHint(adminOperatorsRevokeCmd); got != "" {
		t.Errorf("revoke has no conflict to explain, got %q", got)
	}
}
