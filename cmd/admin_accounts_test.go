package cmd

import (
	"net/http"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/spf13/cobra"
)

// setFlags sets flags the way a command line would (so Changed is true) and puts them
// back when the test ends.
func setFlags(t *testing.T, cmd *cobra.Command, values map[string]string) {
	t.Helper()
	for name, value := range values {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("%s has no --%s", cmd.CommandPath(), name)
		}
		if err := f.Value.Set(value); err != nil {
			t.Fatalf("--%s=%s: %v", name, value, err)
		}
		f.Changed = true
		t.Cleanup(func() {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	}
}

// answerYes runs the test as if --yes had been passed.
func answerYes(t *testing.T) {
	t.Helper()
	assumeYes = true
	t.Cleanup(func() { assumeYes = false })
}

// accountGateway answers the account routes: the record, its exposure, and the switch.
func accountGateway(t *testing.T, active bool, exposure map[string]any) *operatorGateway {
	t.Helper()
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		switch {
		case r.URL.Path == "/admin/accounts/acc-9/exposure":
			return http.StatusOK, exposure
		case r.URL.Path == "/accounts/acc-9" && r.Method == http.MethodGet:
			return http.StatusOK, map[string]any{"id": "acc-9", "name": "Acme", "active": active}
		case r.URL.Path == "/accounts/acc-9" && r.Method == http.MethodPatch:
			return http.StatusOK, map[string]any{"id": "acc-9", "name": "Acme", "active": !active}
		}
		return 0, nil
	}
	return g
}

var (
	noExposure = map[string]any{
		"payments": map[string]any{"in_flight": 0, "in_escrow": 0, "disputed": 0, "by_token": []any{}},
		"payouts":  map[string]any{"pending": 0, "submitted": 0}, "empty": true,
	}
	openExposure = map[string]any{
		"payments": map[string]any{"in_flight": 1, "in_escrow": 2, "disputed": 0, "by_token": []any{
			map[string]any{"chain_id": nil, "token": "0x3600000000000000000000000000000000000000", "symbol": "USDC",
				"count": 3, "amount": "125500000", "amount_formatted": "125.5"},
		}},
		"payouts": map[string]any{"pending": 0, "submitted": 1}, "empty": false,
	}
)

func TestAdminAccountsListSendsItsFilters(t *testing.T) {
	g := newOperatorGateway(t)
	setFlags(t, adminAccountsCmd, map[string]string{"search": " 0x2D2B ", "status": "deactivated", "operator": "false"})
	if _, err := runE(t, adminAccountsCmd); err != nil {
		t.Fatalf("accounts: %v", err)
	}
	q := g.last(t, http.MethodGet, "/accounts").query
	if q.Get("q") != "0x2D2B" || q.Get("status") != "deactivated" || q.Get("admin") != "false" {
		t.Errorf("query = %v", q)
	}

	setFlags(t, adminAccountsCmd, map[string]string{"operator": "maybe"})
	if _, err := runE(t, adminAccountsCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("--operator maybe: err = %v, want a usage error", err)
	}
}

// A write is refused here, with its own code, before the account is even read.
func TestAdminAccountsDeactivateNeedsWriteMode(t *testing.T) {
	g := accountGateway(t, true, noExposure)
	answerYes(t)
	_, err := runE(t, adminAccountsDeactivateCmd, "acc-9")
	if code := agentErrorCode(err, exitCode(err)); err == nil || code != "write_mode_required" {
		t.Fatalf("err = %v (%s), want write_mode_required", err, code)
	}
	if n := len(g.requests()); n != 0 {
		t.Errorf("%d requests were sent without write mode", n)
	}
}

func TestAdminAccountsDeactivateWithNothingInMotion(t *testing.T) {
	g := accountGateway(t, true, noExposure)
	wt := writeModeOn(t)
	answerYes(t)
	out, err := runE(t, adminAccountsDeactivateCmd, "acc-9")
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	req := g.last(t, http.MethodPatch, "/accounts/acc-9")
	if req.token != wt {
		t.Errorf("the switch presented %q, want the write-mode token", req.token)
	}
	if body := req.json(t); len(body) != 1 || body["active"] != false {
		t.Errorf("body = %v, want only active:false", body)
	}
	if !strings.Contains(out, `"active": false`) {
		t.Errorf("output = %s", out)
	}
}

// --yes alone never switches off an account that has something in motion: where no
// question shows the figures, the acknowledgement has to be the flag.
func TestAdminAccountsDeactivateWithOpenExposureNeedsTheAcknowledgement(t *testing.T) {
	g := accountGateway(t, true, openExposure)
	writeModeOn(t)
	answerYes(t)
	_, err := runE(t, adminAccountsDeactivateCmd, "acc-9")
	if code := agentErrorCode(err, exitCode(err)); err == nil || code != "exposure_not_acknowledged" || exitCode(err) != ExitCodeUsage {
		t.Fatalf("err = %v (%s), want exposure_not_acknowledged", err, code)
	}
	for _, want := range []string{`"Acme" (acc-9)`, "1 payment in flight", "2 payments in escrow", "1 payout submitted", "125.5 USDC", "--acknowledge-exposure"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not say %q", err.Error(), want)
		}
	}
	if g.sent(http.MethodPatch, "/accounts/acc-9") {
		t.Fatal("the account was switched off without the acknowledgement")
	}

	setFlags(t, adminAccountsDeactivateCmd, map[string]string{"acknowledge-exposure": "true"})
	if _, err := runE(t, adminAccountsDeactivateCmd, "acc-9"); err != nil {
		t.Fatalf("with the acknowledgement: %v", err)
	}
	if body := g.last(t, http.MethodPatch, "/accounts/acc-9").json(t); body["active"] != false || body["acknowledge_open_exposure"] != true {
		t.Errorf("body = %v", body)
	}
}

// Without --yes and without a terminal there is no question to answer: the command stops,
// for an account with nothing in motion too.
func TestAdminAccountsDeactivateAsksFirst(t *testing.T) {
	g := accountGateway(t, true, noExposure)
	writeModeOn(t)
	_, err := runE(t, adminAccountsDeactivateCmd, "acc-9")
	if code := agentErrorCode(err, exitCode(err)); err == nil || code != "confirmation_required" {
		t.Fatalf("err = %v (%s), want confirmation_required", err, code)
	}
	if g.sent(http.MethodPatch, "/accounts/acc-9") {
		t.Error("the account was switched off without a confirmation")
	}
}

func TestAdminAccountsDeactivateLeavesADeactivatedAccountAlone(t *testing.T) {
	g := accountGateway(t, false, openExposure)
	writeModeOn(t)
	answerYes(t)
	var out string
	var err error
	stderr := captureStderr(t, func() { out, err = runE(t, adminAccountsDeactivateCmd, "acc-9") })
	if err != nil || !strings.Contains(stderr, "already deactivated") || !strings.Contains(out, `"id": "acc-9"`) {
		t.Errorf("err = %v, stderr = %q, out = %s", err, stderr, out)
	}
	if g.sent(http.MethodPatch, "/accounts/acc-9") {
		t.Error("a deactivated account was written again")
	}
}

func TestAdminAccountsActivate(t *testing.T) {
	g := accountGateway(t, false, noExposure)
	if _, err := runE(t, adminAccountsActivateCmd, "acc-9"); err == nil {
		t.Fatal("activate ran without write mode")
	}
	wt := writeModeOn(t)
	if _, err := runE(t, adminAccountsActivateCmd, "acc-9"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	req := g.last(t, http.MethodPatch, "/accounts/acc-9")
	if body := req.json(t); req.token != wt || len(body) != 1 || body["active"] != true {
		t.Errorf("token %q, body %v", req.token, body)
	}
}

// The question names the account and says what it still has in motion, in words.
func TestDeactivateAccountPrompt(t *testing.T) {
	isolate(t)
	nothing := &rail0.AccountExposure{Empty: true}
	if got := deactivateAccountPrompt(&rail0.AccountFull{ID: "acc-9", Name: "Acme"}, nothing); !strings.Contains(got, `Deactivate account "Acme" (acc-9)?`) || !strings.Contains(got, "nothing in motion") {
		t.Errorf("prompt = %q", got)
	}
	open := &rail0.AccountExposure{
		Payments: rail0.ExposurePayments{InFlight: 1, InEscrow: 2, ByToken: []rail0.ExposureToken{
			{Token: "0x3600000000000000000000000000000000000000", Symbol: "USDC", Count: 3, Amount: "125500000", AmountFormatted: "125.5"},
			{Token: "0x00000000000000000000000000000000DeaDBeef", Count: 1, Amount: "7", AmountFormatted: "7"},
		}},
		Payouts: rail0.ExposurePayouts{Submitted: 1},
	}
	got := deactivateAccountPrompt(&rail0.AccountFull{ID: "acc-9"}, open)
	for _, want := range []string{
		"Deactivate account acc-9?",
		"1 payment in flight, 2 payments in escrow, 1 payout submitted (125.5 USDC, 7 0x0000…Beef)",
		"in flight can no longer be authorized or charged",
		"reversible",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt %q does not say %q", got, want)
		}
	}
}

// ── wallets ──────────────────────────────────────────────────────────────────

// The account is always named: an operator's own is never the default.
func TestAdminWalletsNeedAnAccount(t *testing.T) {
	g := newOperatorGateway(t)
	writeModeOn(t)
	answerYes(t)
	for _, c := range []*cobra.Command{adminWalletsCmd, adminWalletsUsageCmd, adminWalletsDeactivateCmd, adminWalletsActivateCmd} {
		if _, err := runE(t, c, "0x2d2b"); err == nil || exitCode(err) != ExitCodeUsage || !strings.Contains(err.Error(), "--account") {
			t.Errorf("%s without --account: err = %v", c.CommandPath(), err)
		}
	}
	if n := len(g.requests()); n != 0 {
		t.Errorf("%d requests were sent with no account named", n)
	}
}

func TestAdminWalletsSwitch(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		switch r.URL.Path {
		case "/accounts/acc-9/wallets/w-1/usage":
			return http.StatusOK, map[string]any{"wallet_id": "w-1", "address": "0x2d2b", "payments_as_payee": 12, "payments_as_payer": 1, "transactions_as_sender": 30}
		case "/accounts/acc-9/wallets/w-1":
			return http.StatusOK, map[string]any{"id": "w-1", "address": "0x2d2b", "active": false, "deactivated_by": "operator"}
		}
		return 0, nil
	}
	adminWalletsAccount = "acc-9"
	t.Cleanup(func() { adminWalletsAccount = "" })

	if _, err := runE(t, adminWalletsDeactivateCmd, "w-1"); agentErrorCode(err, exitCode(err)) != "write_mode_required" {
		t.Fatalf("without write mode: err = %v", err)
	}
	wt := writeModeOn(t)
	if _, err := runE(t, adminWalletsDeactivateCmd, "w-1"); agentErrorCode(err, exitCode(err)) != "confirmation_required" {
		t.Fatalf("without --yes: err = %v", err)
	}
	if !g.sent(http.MethodGet, "/accounts/acc-9/wallets/w-1/usage") || g.sent(http.MethodPatch, "/accounts/acc-9/wallets/w-1") {
		t.Fatal("the usage must be read before the question, and nothing written without an answer")
	}

	answerYes(t)
	out, err := runE(t, adminWalletsDeactivateCmd, "w-1")
	if err != nil || !strings.Contains(out, `"deactivated_by": "operator"`) {
		t.Fatalf("deactivate: %v\n%s", err, out)
	}
	req := g.last(t, http.MethodPatch, "/accounts/acc-9/wallets/w-1")
	if body := req.json(t); req.token != wt || len(body) != 1 || body["active"] != false {
		t.Errorf("token %q, body %v", req.token, body)
	}

	if _, err := runE(t, adminWalletsActivateCmd, "w-1"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if body := g.last(t, http.MethodPatch, "/accounts/acc-9/wallets/w-1").json(t); body["active"] != true {
		t.Errorf("activate body = %v", body)
	}

	if _, err := runE(t, adminWalletsCmd); err != nil {
		t.Fatalf("list: %v", err)
	}
	g.last(t, http.MethodGet, "/accounts/acc-9/wallets")
}

// ── onboardings ──────────────────────────────────────────────────────────────

func TestAdminOnboardingsListSendsItsFilters(t *testing.T) {
	g := newOperatorGateway(t)
	if _, err := runE(t, adminOnboardingsCmd); err != nil {
		t.Fatalf("onboardings: %v", err)
	}
	if q := g.last(t, http.MethodGet, "/admin/onboardings").query; len(q) != 0 {
		t.Errorf("bare list sent %v: the default states are the gateway's", q)
	}

	setFlags(t, adminOnboardingsCmd, map[string]string{"status": "open,completed", "origin": "signup", "send-failed": "true"})
	if _, err := runE(t, adminOnboardingsCmd); err != nil {
		t.Fatalf("onboardings: %v", err)
	}
	q := g.last(t, http.MethodGet, "/admin/onboardings").query
	if q.Get("status") != "open,completed" || q.Get("origin") != "signup" || q.Get("send_failed") != "true" {
		t.Errorf("query = %v", q)
	}
}

const testMerchantAddress = "0x2D2B00000000000000000000000000000000aBcD"

func TestAdminOnboardingsInvite(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		if r.URL.Path == "/signup" {
			return http.StatusCreated, map[string]any{
				"onboarding": map[string]any{"id": "ob-1", "status": "open", "origin": "operator"}, "conflicts": []any{},
				"email": map[string]any{"kind": "invite", "to": "ops@acme.example", "queued": true},
			}
		}
		return 0, nil
	}
	wt := writeModeOn(t)

	setFlags(t, adminOnboardingsInviteCmd, map[string]string{"name": "Acme", "email": "ops@acme.example"})
	if _, err := runE(t, adminOnboardingsInviteCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("without --address: err = %v", err)
	}
	setFlags(t, adminOnboardingsInviteCmd, map[string]string{"address": "0x2D2B"})
	if _, err := runE(t, adminOnboardingsInviteCmd); err == nil || !strings.Contains(err.Error(), "not a wallet address") {
		t.Errorf("a short address: err = %v", err)
	}
	if g.sent(http.MethodPost, "/signup") {
		t.Fatal("an invite went out with a value missing or wrong")
	}

	setFlags(t, adminOnboardingsInviteCmd, map[string]string{"address": testMerchantAddress})
	out, err := runE(t, adminOnboardingsInviteCmd)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	req := g.last(t, http.MethodPost, "/signup")
	if req.token != wt {
		t.Errorf("the invite presented %q, want the write-mode token", req.token)
	}
	if body := req.json(t); body["name"] != "Acme" || body["email"] != "ops@acme.example" || body["address"] != testMerchantAddress {
		t.Errorf("body = %v", body)
	}
	if !strings.Contains(out, `"kind": "invite"`) {
		t.Errorf("output = %s", out)
	}
}

// Without write mode the invite must not reach /signup at all.
func TestAdminOnboardingsInviteNeedsWriteMode(t *testing.T) {
	g := newOperatorGateway(t)
	setFlags(t, adminOnboardingsInviteCmd, map[string]string{"name": "Acme", "email": "ops@acme.example", "address": testMerchantAddress})
	if _, err := runE(t, adminOnboardingsInviteCmd); agentErrorCode(err, exitCode(err)) != "write_mode_required" {
		t.Fatalf("err = %v, want write_mode_required", err)
	}
	if n := len(g.requests()); n != 0 {
		t.Errorf("%d requests were sent without write mode", n)
	}
}

func TestAdminOnboardingsCorrectResendCancel(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		switch {
		case r.Method == http.MethodDelete:
			return http.StatusNoContent, nil
		case strings.HasPrefix(r.URL.Path, "/admin/onboardings/ob-1"):
			return http.StatusOK, map[string]any{"onboarding": map[string]any{"id": "ob-1"},
				"email": map[string]any{"kind": "correction", "to": "right@acme.example", "queued": true}}
		}
		return 0, nil
	}
	wt := writeModeOn(t)

	if _, err := runE(t, adminOnboardingsCorrectCmd, "ob-1"); err == nil || !strings.Contains(err.Error(), "nothing to correct") {
		t.Errorf("correct with no value: err = %v", err)
	}
	setFlags(t, adminOnboardingsCorrectCmd, map[string]string{"email": "right@acme.example"})
	if _, err := runE(t, adminOnboardingsCorrectCmd, "ob-1"); err != nil {
		t.Fatalf("correct: %v", err)
	}
	req := g.last(t, http.MethodPatch, "/admin/onboardings/ob-1")
	if body := req.json(t); req.token != wt || len(body) != 1 || body["email"] != "right@acme.example" {
		t.Errorf("token %q, body %v: only the corrected value goes out", req.token, body)
	}

	if _, err := runE(t, adminOnboardingsResendCmd, "ob-1"); err != nil {
		t.Fatalf("resend: %v", err)
	}
	g.last(t, http.MethodPost, "/admin/onboardings/ob-1/send")

	if _, err := runE(t, adminOnboardingsCancelCmd, "ob-1"); agentErrorCode(err, exitCode(err)) != "confirmation_required" {
		t.Fatalf("cancel without --yes: err = %v", err)
	}
	if g.sent(http.MethodDelete, "/admin/onboardings/ob-1") {
		t.Fatal("a request was cancelled without a confirmation")
	}
	answerYes(t)
	out, err := runE(t, adminOnboardingsCancelCmd, "ob-1")
	if err != nil || !strings.Contains(out, `"cancelled": true`) {
		t.Fatalf("cancel: %v\n%s", err, out)
	}
	if req := g.last(t, http.MethodDelete, "/admin/onboardings/ob-1"); req.token != wt {
		t.Errorf("cancel presented %q, want the write-mode token", req.token)
	}
}
