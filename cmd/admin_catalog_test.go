package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

// configGateway answers the configuration report the way the gateway does: one setting
// for each source it names, and one group from a gateway older than the field.
func configGateway(t *testing.T) *operatorGateway {
	t.Helper()
	g := newOperatorGateway(t)
	setting := func(value any, env any, overridden bool, source string) map[string]any {
		s := map[string]any{"value": value, "env": env, "overridden": overridden, "description": "what it is for"}
		if source != "" {
			s["source"] = source
		}
		return s
	}
	g.respond = func(r *http.Request) (int, any) {
		if r.URL.Path == "/admin/config" {
			return http.StatusOK, map[string]any{
				"build":        map[string]any{"version": setting("1.0.1", nil, false, "")},
				"payouts":      map[string]any{"enabled": setting(false, "RAIL0_FEATURE_PAYOUTS", false, "default"), "proof_amount": setting("0.01", nil, false, "constant")},
				"rate_limit":   map[string]any{"period": setting(60, "RATE_LIMIT_PERIOD", false, "default")},
				"recovery":     map[string]any{"recovery_interval": setting(120, "RECOVERY_INTERVAL", true, "override")},
				"siwe":         map[string]any{"domains": setting([]any{"console.example", "gateway.example"}, "SIWE_DOMAIN", true, "a_later_source")},
				"fees":         map[string]any{"multipliers": setting(map[string]any{"137": 1.5}, nil, false, "constant")},
				"generated_at": "2026-11-27T15:00:00Z",
			}
		}
		return 0, nil
	}
	return g
}

func configOutput(t *testing.T, args ...string) []map[string]any {
	t.Helper()
	out, err := runE(t, adminConfigCmd, args...)
	if err != nil {
		t.Fatalf("admin config %v: %v", args, err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output is not a list of rows: %v\n%s", err, out)
	}
	return rows
}

func configRow(t *testing.T, rows []map[string]any, group, setting string) map[string]any {
	t.Helper()
	for _, r := range rows {
		if r["group"] == group && r["setting"] == setting {
			return r
		}
	}
	t.Fatalf("no row %s.%s in %v", group, setting, rows)
	return nil
}

// The report is printed one setting a row, sorted, with nothing but generated_at left out.
func TestAdminConfigOneSettingPerRow(t *testing.T) {
	configGateway(t)
	rows := configOutput(t)
	var order []string
	for _, r := range rows {
		order = append(order, r["group"].(string)+"."+r["setting"].(string))
	}
	if got := strings.Join(order, " "); got != "build.version fees.multipliers payouts.enabled payouts.proof_amount rate_limit.period recovery.recovery_interval siwe.domains" {
		t.Errorf("order = %s", got)
	}
	interval := configRow(t, rows, "recovery", "recovery_interval")
	if interval["value"] != float64(120) || interval["env"] != "RECOVERY_INTERVAL" || interval["overridden"] != true || interval["description"] != "what it is for" {
		t.Errorf("recovery.recovery_interval = %v", interval)
	}
}

// Where a value comes from is the gateway's word, passed on as it is, also when it is a
// word this version does not know; a gateway older than the field leaves the key in the
// row with nothing in it, so the shape of a row does not depend on the gateway.
func TestAdminConfigSource(t *testing.T) {
	configGateway(t)
	rows := configOutput(t)
	for _, tc := range []struct{ group, setting, source string }{
		{"payouts", "proof_amount", "constant"},
		{"rate_limit", "period", "default"},
		{"recovery", "recovery_interval", "override"},
		{"siwe", "domains", "a_later_source"},
	} {
		if got := configRow(t, rows, tc.group, tc.setting)["source"]; got != tc.source {
			t.Errorf("%s.%s source = %v, want %s", tc.group, tc.setting, got, tc.source)
		}
	}
	old := configRow(t, rows, "build", "version")
	if source, present := old["source"]; !present || source != nil {
		t.Errorf("a gateway that names no source: source = %v (present %v), want a null", source, present)
	}
}

func TestAdminConfigGroupAndOverridden(t *testing.T) {
	configGateway(t)
	if rows := configOutput(t, "payouts"); len(rows) != 2 || rows[0]["group"] != "payouts" {
		t.Errorf("config payouts = %v", rows)
	}
	if _, err := runE(t, adminConfigCmd, "paiouts"); err == nil || exitCode(err) != ExitCodeUsage || !strings.Contains(err.Error(), "build, fees, payouts, rate_limit, recovery, siwe") {
		t.Errorf("an unknown group: err = %v", err)
	}
	adminConfigOverridden = true
	t.Cleanup(func() { adminConfigOverridden = false })
	rows := configOutput(t)
	if len(rows) != 2 || rows[0]["setting"] != "recovery_interval" || rows[1]["setting"] != "domains" {
		t.Errorf("--overridden = %v", rows)
	}
	if rows := configOutput(t, "payouts"); len(rows) != 0 {
		t.Errorf("--overridden on a group with none = %v", rows)
	}
}

func TestConfigCells(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want string
	}{{nil, ""}, {"USDC", "USDC"}, {float64(604800), "604800"}, {false, "false"}, {map[string]any{"137": 1.5}, `{"137":1.5}`}, {[]any{"a", "b"}, `["a","b"]`}} {
		if got := compactJSON(tc.in); got != tc.want {
			t.Errorf("compactJSON(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if yesNo("active")(map[string]any{"active": false}) != "no" || yesNo("active")(map[string]any{"active": true}) != "yes" || yesNo("active")(map[string]any{}) != "" {
		t.Error("yesNo must say yes, no, and nothing when the row has no answer")
	}
}

// catalogGateway answers the catalogue routes.
func catalogGateway(t *testing.T, pending int) *operatorGateway {
	t.Helper()
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		switch r.URL.Path {
		case "/admin/catalog":
			return http.StatusOK, map[string]any{"stale": pending > 0, "pending": pending, "loaded_at": "2026-11-26T08:00:00Z",
				"blockchains": map[string]any{"added": []any{}, "removed": []any{}, "changed": []any{}}}
		case "/admin/catalog/reload":
			return http.StatusOK, map[string]any{"reloaded": true, "blockchains": 2, "tokens": 3, "contracts": 1, "payment_blockchains": 1,
				"applied": map[string]any{"pending": pending}}
		case "/admin/blockchains":
			return http.StatusOK, []any{
				map[string]any{"id": "b-1", "chain_id": 5042002, "slug": "arc", "active": true, "rpc_urls": []any{"https://lb.example/rpc?dkey={RAIL0_RPC_KEY_DRPC}", "https://rpc.example"}},
				map[string]any{"id": "b-2", "chain_id": 84532, "slug": "base", "active": false, "rpc_urls": []any{"https://base.example"}},
			}
		case "/admin/tokens":
			return http.StatusOK, []any{
				map[string]any{"id": "t-1", "blockchain_id": "b-2", "symbol": "USDC", "eip712_name": "USDC", "active": true},
				map[string]any{"id": "t-2", "blockchain_id": "b-gone", "symbol": "EURC", "active": false},
			}
		case "/admin/contracts":
			return http.StatusOK, []any{map[string]any{"id": "c-1", "blockchain_id": "b-1", "address": "0x13a4", "version": "1.4.0", "active": true}}
		}
		return 0, nil
	}
	return g
}

func TestAdminCatalogDiffAndRows(t *testing.T) {
	g := catalogGateway(t, 2)
	out, err := runE(t, adminCatalogCmd)
	if err != nil || !strings.Contains(out, `"stale": true`) || !strings.Contains(out, `"pending": 2`) {
		t.Fatalf("catalog: %v\n%s", err, out)
	}
	// Nothing pending is said as 0, not left out.
	g.respond = func(r *http.Request) (int, any) {
		return http.StatusOK, map[string]any{"stale": false, "pending": 0}
	}
	if out, err = runE(t, adminCatalogCmd); err != nil || !strings.Contains(out, `"pending": 0`) || !strings.Contains(out, `"stale": false`) {
		t.Fatalf("catalog with nothing pending: %v\n%s", err, out)
	}
	g = catalogGateway(t, 2)
	if got := catalogChainCell(map[string]any{"chain_slug": "base", "chain_id": float64(84532)}); got != "base" {
		t.Errorf("chain cell = %q", got)
	}
	if got := catalogChainCell(map[string]any{"chain_id": float64(84532)}); got != "84532" {
		t.Errorf("chain cell without a slug = %q", got)
	}

	out, err = runE(t, adminCatalogChainsCmd)
	if err != nil || !strings.Contains(out, `"slug": "base"`) || !strings.Contains(out, "{RAIL0_RPC_KEY_DRPC}") {
		t.Fatalf("catalog chains: %v\n%s", err, out)
	}

	// A token names its chain by the catalogue's reference: the command adds chain_id
	// from the chain list, and leaves a row whose chain is not listed as it came.
	out, err = runE(t, adminCatalogTokensCmd)
	if err != nil {
		t.Fatalf("catalog tokens: %v", err)
	}
	var tokens []map[string]any
	if json.Unmarshal([]byte(out), &tokens) != nil || len(tokens) != 2 || tokens[0]["chain_id"] != float64(84532) ||
		tokens[0]["chain_slug"] != "base" || tokens[1]["chain_id"] != nil || tokens[1]["chain_slug"] != nil {
		t.Errorf("tokens = %s", out)
	}
	if !g.sent(http.MethodGet, "/admin/blockchains") {
		t.Error("the chain list was not read for the tokens")
	}
	out, err = runE(t, adminCatalogContractsCmd)
	if err != nil || !strings.Contains(out, `"chain_id": 5042002`) || !strings.Contains(out, `"version": "1.4.0"`) {
		t.Errorf("catalog contracts: %v\n%s", err, out)
	}
}

// The reload: write mode first, then the diff is read and the question says how much
// would change; only a yes sends it, with the write-mode token.
func TestAdminCatalogReload(t *testing.T) {
	g := catalogGateway(t, 2)
	if _, err := runE(t, adminCatalogReloadCmd); agentErrorCode(err, exitCode(err)) != "write_mode_required" {
		t.Fatalf("without write mode: err = %v", err)
	}
	if n := len(g.requests()); n != 0 {
		t.Fatalf("%d requests were sent without write mode", n)
	}
	wt := writeModeOn(t)
	if _, err := runE(t, adminCatalogReloadCmd); agentErrorCode(err, exitCode(err)) != "confirmation_required" {
		t.Fatalf("without --yes: err = %v", err)
	}
	if !g.sent(http.MethodGet, "/admin/catalog") || g.sent(http.MethodPost, "/admin/catalog/reload") {
		t.Fatal("the diff must be read before the question, and nothing reloaded without an answer")
	}
	answerYes(t)
	out, err := runE(t, adminCatalogReloadCmd)
	if err != nil || !strings.Contains(out, `"reloaded": true`) || !strings.Contains(out, `"payment_blockchains": 1`) {
		t.Fatalf("reload: %v\n%s", err, out)
	}
	if req := g.last(t, http.MethodPost, "/admin/catalog/reload"); req.token != wt {
		t.Errorf("the reload presented %q, want the write-mode token", req.token)
	}

	for pending, want := range map[int]string{0: "changes nothing", 1: "1 change is pending", 3: "3 changes are pending"} {
		got := reloadCatalogPrompt(&rail0.CatalogDiff{Pending: pending})
		if !strings.Contains(got, want) || !strings.Contains(got, "one process only") {
			t.Errorf("prompt for %d pending = %q", pending, got)
		}
	}
}

func TestAdminPayoutsAndDestinations(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		switch r.URL.Path {
		case "/admin/payouts/po-1":
			return http.StatusOK, map[string]any{"id": "po-1", "status": "failed", "account_id": "acc-9", "error_reason": "replaced"}
		case "/admin/payout_destinations/d-1":
			return http.StatusOK, map[string]any{"id": "d-1", "status": "verified", "account_id": "acc-9", "verified_by": "0x2d2b"}
		}
		return 0, nil
	}
	setFlags(t, adminPayoutsCmd, map[string]string{
		"account": "acc-9", "wallet": "w-1", "destination": "d-1", "status": "failed,submitted", "purpose": "payout",
		"created-from": "2026-11-01T00:00:00Z", "created-to": "2026-11-27T00:00:00Z",
	})
	adminPoChainID = 5042
	t.Cleanup(func() { adminPoChainID = 0 })
	if _, err := runE(t, adminPayoutsCmd); err != nil {
		t.Fatalf("payouts: %v", err)
	}
	q := g.last(t, http.MethodGet, "/admin/payouts").query
	for key, want := range map[string]string{
		"account_id": "acc-9", "wallet_id": "w-1", "destination_id": "d-1", "chain_id": "5042", "status": "failed,submitted",
		"purpose": "payout", "created_from": "2026-11-01T00:00:00Z", "created_to": "2026-11-27T00:00:00Z",
	} {
		if q.Get(key) != want {
			t.Errorf("query %s = %q, want %q", key, q.Get(key), want)
		}
	}
	setFlags(t, adminPayoutsCmd, map[string]string{"status": "lost"})
	if _, err := runE(t, adminPayoutsCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("--status lost: err = %v, want a usage error", err)
	}

	if out, err := runE(t, adminPayoutsGetCmd, "po-1"); err != nil || !strings.Contains(out, `"error_reason": "replaced"`) {
		t.Errorf("payouts get: %v\n%s", err, out)
	}
	if _, err := runE(t, adminPayoutsEventsCmd, "po-1"); err != nil || !g.sent(http.MethodGet, "/admin/payouts/po-1/events") {
		t.Errorf("payouts events: %v", err)
	}

	setFlags(t, adminPayoutDestinationsCmd, map[string]string{"account": "acc-9", "wallet": "w-1", "status": "unverified"})
	if _, err := runE(t, adminPayoutDestinationsCmd); err != nil {
		t.Fatalf("destinations: %v", err)
	}
	q = g.last(t, http.MethodGet, "/admin/payout_destinations").query
	if q.Get("account_id") != "acc-9" || q.Get("wallet_id") != "w-1" || q.Get("status") != "unverified" {
		t.Errorf("destinations query = %v", q)
	}
	setFlags(t, adminPayoutDestinationsCmd, map[string]string{"status": "gone"})
	if _, err := runE(t, adminPayoutDestinationsCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("--status gone: err = %v, want a usage error", err)
	}
	if out, err := runE(t, adminPayoutDestinationsGetCmd, "d-1"); err != nil || !strings.Contains(out, `"verified_by": "0x2d2b"`) {
		t.Errorf("destinations get: %v\n%s", err, out)
	}
	if _, err := runE(t, adminPayoutDestinationsEventsCmd, "d-1"); err != nil || !g.sent(http.MethodGet, "/admin/payout_destinations/d-1/events") {
		t.Errorf("destinations events: %v", err)
	}
}

// With the payouts feature off the gateway answers 404 on every one of these routes: the
// hint says so, where "the id may be wrong" alone would send the operator after an id.
func TestAdminPayouts404NamesTheFeature(t *testing.T) {
	for _, cmd := range []struct {
		name string
		hint string
	}{
		{"payouts", notFoundHint(adminPayoutsCmd)},
		{"payouts get", notFoundHint(adminPayoutsGetCmd)},
		{"destinations", notFoundHint(adminPayoutDestinationsCmd)},
		{"destinations events", notFoundHint(adminPayoutDestinationsEventsCmd)},
	} {
		if !strings.Contains(cmd.hint, "payouts feature is off") {
			t.Errorf("%s: hint = %q", cmd.name, cmd.hint)
		}
	}
	if strings.Contains(notFoundHint(adminAccountsGetCmd), "payouts feature") {
		t.Error("the payouts hint leaked to another admin command")
	}
}
