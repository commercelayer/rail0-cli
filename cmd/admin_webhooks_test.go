package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestAdminWebhooksListSendsItsFilters(t *testing.T) {
	g := newOperatorGateway(t)
	setFlags(t, adminWebhooksCmd, map[string]string{"account": "acc-9", "circuit": "open", "topic": "payments.captured"})
	if _, err := runE(t, adminWebhooksCmd); err != nil {
		t.Fatalf("webhooks: %v", err)
	}
	q := g.last(t, http.MethodGet, "/admin/webhooks").query
	if q.Get("account_id") != "acc-9" || q.Get("circuit_state") != "open" || q.Get("topic") != "payments.captured" {
		t.Errorf("query = %v", q)
	}

	setFlags(t, adminWebhooksCmd, map[string]string{"circuit": "half-open"})
	if _, err := runE(t, adminWebhooksCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("--circuit half-open: err = %v, want a usage error", err)
	}
}

// The reset re-arms delivery to someone else's endpoint: write mode, and the write-mode
// token on the request.
func TestAdminWebhooksResetCircuit(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		if r.Method == http.MethodPut {
			return http.StatusOK, map[string]any{"id": "wh-1", "active": true, "circuit_state": "closed", "account_id": "acc-9"}
		}
		return 0, nil
	}
	if _, err := runE(t, adminWebhooksResetCircuitCmd, "wh-1"); agentErrorCode(err, exitCode(err)) != "write_mode_required" {
		t.Fatalf("without write mode: err = %v", err)
	}
	if n := len(g.requests()); n != 0 {
		t.Fatalf("%d requests were sent without write mode", n)
	}
	wt := writeModeOn(t)
	out, err := runE(t, adminWebhooksResetCircuitCmd, "wh-1")
	if err != nil || !strings.Contains(out, `"circuit_state": "closed"`) {
		t.Fatalf("reset-circuit: %v\n%s", err, out)
	}
	if req := g.last(t, http.MethodPut, "/admin/webhooks/wh-1/reset_circuit"); req.token != wt {
		t.Errorf("the reset presented %q, want the write-mode token", req.token)
	}
}

func TestAdminDeliveries(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		if r.URL.Path == "/admin/event_callbacks" {
			return http.StatusOK, []any{map[string]any{
				"id": "cb-1", "topic": "payments.captured", "status": "failed", "response_code": 500,
				"payload": `{"event":"the whole body"}`,
				"webhook": map[string]any{"id": "wh-1", "name": "Shop", "account_id": "acc-9", "account": map[string]any{"name": "Acme"}},
			}}
		}
		return 0, nil
	}
	rail0ID := "0x" + strings.Repeat("ab", 32)
	setFlags(t, adminDeliveriesCmd, map[string]string{
		"payment": rail0ID, "webhook": "wh-1", "status": "failed", "topic": "payments.captured",
		"response-code": "500", "since": "2026-11-26T00:00:00Z", "until": "2026-11-27T00:00:00Z",
	})
	out, err := runE(t, adminDeliveriesCmd)
	if err != nil {
		t.Fatalf("deliveries: %v", err)
	}
	q := g.last(t, http.MethodGet, "/admin/event_callbacks").query
	for key, want := range map[string]string{
		"payment_id": rail0ID, "webhook_id": "wh-1", "status": "failed", "topic": "payments.captured",
		"response_code": "500", "since": "2026-11-26T00:00:00Z", "until": "2026-11-27T00:00:00Z",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	if strings.Contains(out, "the whole body") || !strings.Contains(out, `"name": "Shop"`) {
		t.Errorf("the payload must be left out and the subscription kept:\n%s", out)
	}

	setFlags(t, adminDeliveriesCmd, map[string]string{"payload": "true"})
	if out, err = runE(t, adminDeliveriesCmd); err != nil || !strings.Contains(out, "the whole body") {
		t.Errorf("--payload: %v\n%s", err, out)
	}

	setFlags(t, adminDeliveriesCmd, map[string]string{"payment": "0x1234"})
	if _, err := runE(t, adminDeliveriesCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("a short --payment: err = %v, want a usage error", err)
	}
	setFlags(t, adminDeliveriesCmd, map[string]string{"payment": "", "status": "pending"})
	if _, err := runE(t, adminDeliveriesCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("--status pending: err = %v, want a usage error", err)
	}
}

func TestAdminAudit(t *testing.T) {
	g := newOperatorGateway(t)
	paymentUUID := "019e748b-da9a-7c3f-ba32-50572ffd5388"
	setFlags(t, adminAuditCmd, map[string]string{
		"action": "account.deactivate", "actor": "acc-1", "target": "acc-9", "resource-type": "account",
		"payment": paymentUUID, "from": "2026-11-01T00:00:00Z", "to": "2026-11-27T00:00:00Z", "sort": "action",
	})
	if _, err := runE(t, adminAuditCmd); err != nil {
		t.Fatalf("audit: %v", err)
	}
	q := g.last(t, http.MethodGet, "/admin/actions").query
	for key, want := range map[string]string{
		"action": "account.deactivate", "actor_account_id": "acc-1", "target_account_id": "acc-9",
		"resource_type": "account", "payment_id": paymentUUID, "from": "2026-11-01T00:00:00Z",
		"to": "2026-11-27T00:00:00Z", "sort": "action",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}

	// An action the CLI does not know yet is the gateway's to judge, not refused here.
	before := len(g.requests())
	setFlags(t, adminAuditCmd, map[string]string{"action": "something.new", "payment": ""})
	if _, err := runE(t, adminAuditCmd); err != nil || len(g.requests()) != before+1 {
		t.Errorf("an unknown --action must reach the gateway: err = %v", err)
	}

	// The audit log knows a payment by its UUID only: a rail0_id is refused here, with
	// the way to the same answer.
	before = len(g.requests())
	setFlags(t, adminAuditCmd, map[string]string{"payment": "0x" + strings.Repeat("ab", 32)})
	_, err := runE(t, adminAuditCmd)
	if err == nil || exitCode(err) != ExitCodeUsage || !strings.Contains(err.Error(), "rail0 admin trace") || len(g.requests()) != before {
		t.Errorf("a rail0_id in --payment: err = %v, %d requests sent", err, len(g.requests())-before)
	}
}

// The tables an operator reads: whose row it is comes from the nested account, and a
// detail is shown as it was recorded.
func TestOperatorWebhookAndAuditCells(t *testing.T) {
	webhook := map[string]any{"account_id": "acc-9", "account": map[string]any{"name": "Acme"},
		"last_attempt_at": "2026-11-20T09:59:58Z", "last_attempt_status": "failed"}
	if got := accountName(webhook, "account", "account_id"); got != "Acme" {
		t.Errorf("account = %q", got)
	}
	if got := accountName(map[string]any{"account_id": "acc-9"}, "account", "account_id"); got != "acc-9" {
		t.Errorf("account without a name = %q, want its id", got)
	}
	for _, c := range operatorWebhookColumns {
		if c.Header == "last attempt" {
			if got := c.Value(webhook); !strings.HasPrefix(got, "failed 2026-11-20") {
				t.Errorf("last attempt = %q", got)
			}
			if got := c.Value(map[string]any{}); got != "" {
				t.Errorf("no attempt yet = %q, want empty", got)
			}
		}
	}
	for _, c := range auditColumns {
		switch c.Header {
		case "detail":
			if got := c.Value(map[string]any{"detail": map[string]any{"deleted": float64(3)}}); got != `{"deleted":3}` {
				t.Errorf("detail = %q", got)
			}
			if got := c.Value(map[string]any{}); got != "" {
				t.Errorf("no detail = %q", got)
			}
		case "actor":
			if got := c.Value(map[string]any{"actor_address": "0x134a0f43452debed687720cd343bbaf52a4d378d"}); !strings.HasPrefix(got, "0x134a") {
				t.Errorf("actor without an account = %q, want its address", got)
			}
		}
	}
}
