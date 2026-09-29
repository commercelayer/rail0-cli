package cmd

import (
	"net/http"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

func TestResolveAccountID(t *testing.T) {
	// An explicit --account wins and needs no session.
	if id, err := resolveAccountID("019e748b-acc"); err != nil || id != "019e748b-acc" {
		t.Fatalf("explicit: got %q err %v", id, err)
	}

	// No flag and no session → error, and it points at --account.
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	_, err := resolveAccountID("")
	if err == nil {
		t.Fatal("no flag + no session must error")
	}
	if !strings.Contains(err.Error(), "--account") {
		t.Errorf("error should mention --account, got: %v", err)
	}
}

func TestAccountCommandTree(t *testing.T) {
	// Bare `account` shows the profile, like `account get`; neither takes a positional.
	if accountCmd.RunE == nil {
		t.Error("account should show the profile when run bare (nil RunE)")
	}
	for _, verb := range []string{"get", "update"} {
		sub, _, err := accountCmd.Find([]string{verb})
		if err != nil || sub == nil || sub.Name() != verb {
			t.Fatalf("account %s: not found (%v)", verb, err)
		}
		if err := sub.Args(sub, []string{"x"}); err == nil {
			t.Errorf("account %s should take no positional args", verb)
		}
		// --account is persistent: the same flag on the bare command and both verbs.
		if sub.InheritedFlags().Lookup("account") == nil {
			t.Errorf("account %s: --account should be inherited from 'account'", verb)
		}
	}
	if err := accountCmd.Args(accountCmd, []string{"x"}); err == nil {
		t.Error("bare account should take no positional args")
	}
	// active is the operator's field: the owner-facing CLI must not offer it.
	if accountUpdateCmd.Flags().Lookup("active") != nil {
		t.Error("account update must not expose --active (operator-only)")
	}
	if accountCmd.GroupID != groupAccount {
		t.Errorf("account should be listed under the account help group, got %q", accountCmd.GroupID)
	}
}

// resetAccountFlags clears the package-level flag state between tests: cobra keeps
// both the values and each flag's Changed bit, and accountUpdateRequest reads Changed.
func resetAccountFlags(t *testing.T) {
	t.Helper()
	reset := func() {
		accountID, accountUpdateName, accountUpdateEmail = "", "", ""
		for _, n := range []string{"name", "email"} {
			accountUpdateCmd.Flags().Lookup(n).Changed = false
		}
	}
	reset()
	t.Cleanup(reset)
}

func TestAccountGetUsesTheSessionAccount(t *testing.T) {
	g := newRecordingGateway(t)
	resetAccountFlags(t)
	g.respond = func(*http.Request) any { return map[string]any{"id": "acc-1", "name": "Acme"} }

	run(t, accountCmd) // bare
	g.last(t, http.MethodGet, "/accounts/acc-1")
	run(t, accountGetCmd)
	g.last(t, http.MethodGet, "/accounts/acc-1")
}

func TestAccountGetHonoursExplicitAccount(t *testing.T) {
	g := newRecordingGateway(t)
	resetAccountFlags(t)
	g.respond = func(*http.Request) any { return map[string]any{"id": "acc-other"} }
	accountID = "acc-other"

	run(t, accountGetCmd)
	g.last(t, http.MethodGet, "/accounts/acc-other")
}

func TestAccountUpdateSendsOnlyTheFlagsGiven(t *testing.T) {
	g := newRecordingGateway(t)
	resetAccountFlags(t)
	g.respond = func(*http.Request) any { return map[string]any{"id": "acc-1"} }

	_ = accountUpdateCmd.Flags().Set("email", " ops@acme.example ")
	run(t, accountUpdateCmd)
	body := g.last(t, http.MethodPatch, "/accounts/acc-1").body
	if body["email"] != "ops@acme.example" {
		t.Errorf("email = %v, want the trimmed value", body["email"])
	}
	if _, ok := body["name"]; ok {
		t.Errorf("name was not passed and must not be sent: %v", body)
	}
	if _, ok := body["active"]; ok {
		t.Errorf("active must never be sent: %v", body)
	}

	_ = accountUpdateCmd.Flags().Set("name", "Acme Store")
	run(t, accountUpdateCmd)
	body = g.last(t, http.MethodPatch, "/accounts/acc-1").body
	if body["name"] != "Acme Store" || body["email"] != "ops@acme.example" {
		t.Errorf("body = %v, want both fields", body)
	}
}

// Nothing to change, or a flag passed blank, is refused before any request — the
// gateway would answer 400, and a blank value would otherwise be dropped silently by
// the SDK's omitempty.
func TestAccountUpdateRefusesLocally(t *testing.T) {
	g := newRecordingGateway(t)

	for _, tc := range []struct {
		name  string
		flags map[string]string
		want  string
	}{
		{"no flags", nil, "--name and/or --email"},
		{"blank name", map[string]string{"name": "  "}, "--name cannot be empty"},
		{"blank email beside a name", map[string]string{"name": "Acme", "email": ""}, "--email cannot be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAccountFlags(t)
			for k, v := range tc.flags {
				_ = accountUpdateCmd.Flags().Set(k, v)
			}
			err := accountUpdateCmd.RunE(accountUpdateCmd, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.reqs) != 0 {
		t.Errorf("a locally refused update must send nothing, got %v", g.reqs)
	}
}

// The gateway's 409 comes from the unique index and cannot name the field; the
// command can, and only this command gets that wording.
func TestAccountUpdateConflictNamesTheFields(t *testing.T) {
	conflict := &rail0.APIError{Status: 409, Code: "conflict", Detail: "A resource with those identifying values already exists."}

	got := humanizeErrorFor(accountUpdateCmd, conflict).Error()
	if !strings.Contains(got, "HTTP 409") || !strings.Contains(got, "name or email is already used by another account") {
		t.Errorf("account update 409 = %q", got)
	}
	if other := humanizeErrorFor(walletsCreateCmd, conflict).Error(); strings.Contains(other, "name or email") {
		t.Errorf("the account wording must not leak onto other commands: %q", other)
	}
}

// Not a payments command, so a 404 gets the neutral wording, not payer/payee advice.
func TestAccountNotFoundIsNeutral(t *testing.T) {
	got := humanizeErrorFor(accountGetCmd, &rail0.APIError{Status: 404, Code: "not_found"}).Error()
	if strings.Contains(got, "payer/payee") || !strings.Contains(got, "not visible to this session") {
		t.Errorf("account 404 = %q", got)
	}
}
