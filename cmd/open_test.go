package cmd

import (
	"strings"
	"testing"
)

// The page `rail0 open` builds: the admin's home, or a payment's page — the merchant's
// or, with --console, the operator's — and a refusal for an id the page cannot answer.
func TestAdminPageURL(t *testing.T) {
	const base = "https://rail0-admin-staging.fly.dev"
	rail0ID := "0x" + strings.Repeat("ab", 32)
	uuid := "1b4e28ba-2fa1-41d2-883f-0016d3cca427"
	cases := []struct {
		id      string
		console bool
		want    string
	}{
		{"", false, base + "/"},
		{"", true, base + "/admin"},
		{rail0ID, false, base + "/payments/" + rail0ID},
		{uuid, true, base + "/admin/payments/" + uuid},
	}
	for _, c := range cases {
		got, err := adminPageURL(base, c.id, c.console)
		if err != nil || got != c.want {
			t.Errorf("adminPageURL(%q, console=%v) = %q, %v; want %q", c.id, c.console, got, err, c.want)
		}
	}
	for _, bad := range []string{"0xabc", "ORD-42", "../admin"} {
		if _, err := adminPageURL(base, bad, false); err == nil {
			t.Errorf("adminPageURL(%q) should refuse an id that is neither a UUID nor a rail0_id", bad)
		}
	}
}

// admin-url wins when set; otherwise the environment the gateway identifies supplies
// one (development, staging); production declares none, so the error says how to set it.
func TestAdminURLResolution(t *testing.T) {
	dir := t.TempDir() // an empty config: no stored admin-url or base-url
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("RAIL0_ADMIN_URL", "")
	t.Setenv("RAIL0_BASE_URL", "https://rail0-gateway-staging.fly.dev")
	if got, err := adminURL(); err != nil || got != "https://rail0-admin-staging.fly.dev" {
		t.Errorf("staging gateway: adminURL() = %q, %v", got, err)
	}

	t.Setenv("RAIL0_ADMIN_URL", "https://admin.example.com/")
	if got, err := adminURL(); err != nil || got != "https://admin.example.com" {
		t.Errorf("RAIL0_ADMIN_URL: adminURL() = %q, %v", got, err)
	}

	t.Setenv("RAIL0_ADMIN_URL", "")
	t.Setenv("RAIL0_BASE_URL", "https://api.rail0.xyz")
	if _, err := adminURL(); err == nil || !strings.Contains(err.Error(), "config set admin-url") {
		t.Errorf("production without admin-url should say how to set it, got %v", err)
	}
}

func TestValidateAdminURL(t *testing.T) {
	for _, ok := range []string{"http://localhost:3000", "https://rail0-admin-staging.fly.dev"} {
		if err := validateAdminURL(ok); err != nil {
			t.Errorf("validateAdminURL(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "rail0-admin-staging.fly.dev", "javascript:alert(1)", "file:///etc"} {
		if err := validateAdminURL(bad); err == nil {
			t.Errorf("validateAdminURL(%q) should fail", bad)
		}
	}
}
