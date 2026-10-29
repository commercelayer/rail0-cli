package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	openConsole bool
	openPrint   bool
)

// paymentIDPattern is either identifier the admin's payment page answers to: the
// payment's UUID or its 0x rail0_id. The page resolves both, so the value goes into the
// path unchanged.
var paymentIDPattern = eventCallbackPaymentIDPattern

// openCmd opens the admin app in the browser: its home, or one payment's page.
//
// The admin's address comes from the admin-url setting, or — when that is not set —
// from the environment the gateway setting identifies (development and staging declare
// one), so a config written before admin-url existed still works after `env staging`
// without a second switch. Production declares none, for the reason production
// declares no chain: an address nobody chose is not a default.
var openCmd = &cobra.Command{
	Use:   "open [rail0Id|uuid]",
	Short: "Open the admin app in the browser, on a payment's page when given its id",
	Long: `Open the rail0 admin app in the browser: its home page, or — given a payment's
rail0_id (0x…) or UUID — that payment's page.

--console opens the operator console's page for the payment instead of the merchant's
(it needs an admin session in the browser). --print only prints the URL, as it does
when there is no terminal or with --agent, so a script can pass it on.

The admin's address is the admin-url setting (RAIL0_ADMIN_URL), or else the one the
current environment declares: development and staging have one, production does not
— there, set it with 'rail0 config set admin-url https://…'.`,
	Example: `  rail0 open                          # the admin's home page
  rail0 open 0xabc123…                # that payment's page
  rail0 open 0xabc123… --console      # the console's page for it (operators)
  rail0 open 0xabc123… --print        # just the URL`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		base, err := adminURL()
		if err != nil {
			return err
		}
		id := ""
		if len(args) == 1 {
			id = strings.TrimSpace(args[0])
		}
		target, err := adminPageURL(base, id, openConsole)
		if err != nil {
			return err
		}
		if openPrint || agentMode() || ui.IsMachineOutput() {
			return printLine(target, map[string]any{"url": target})
		}
		if err := openBrowser(target); err != nil {
			return fmt.Errorf("could not open a browser (%v) — the page is %s", err, target)
		}
		fmt.Println(target)
		return nil
	},
}

func init() {
	openCmd.Flags().BoolVar(&openConsole, "console", false, "Open the operator console's page for the payment (admin session)")
	openCmd.Flags().BoolVar(&openPrint, "print", false, "Only print the URL; do not open a browser")
}

// adminURL resolves the admin app's address: the admin-url setting when set, else the
// current environment's declaration, else an error saying how to set one.
func adminURL() (string, error) {
	if v := configuredDefault("admin-url"); v != "" {
		return strings.TrimRight(v, "/"), nil
	}
	if e, err := lookupEnv(detectEnv()); err == nil {
		if v := e.settings["admin-url"]; v != "" {
			return strings.TrimRight(v, "/"), nil
		}
	}
	return "", errors.New("no admin-url for this environment — set one with 'rail0 config set admin-url https://…' (or RAIL0_ADMIN_URL)")
}

// adminPageURL builds the page to open: the admin's home for no id, else the payment's
// page — the merchant's (/payments/<id>) or, with console, the operator's
// (/admin/payments/<id>). An id that is neither a UUID nor a rail0_id is refused here,
// before a browser opens on a page that would only say "not found".
func adminPageURL(base, id string, console bool) (string, error) {
	if id == "" {
		if console {
			return base + "/admin", nil
		}
		return base + "/", nil
	}
	if !paymentIDPattern.MatchString(id) {
		return "", fmt.Errorf("%q is neither a payment UUID nor a rail0_id (0x followed by 64 hex characters)", id)
	}
	path := "/payments/"
	if console {
		path = "/admin/payments/"
	}
	return base + path + url.PathEscape(id), nil
}

// openBrowser hands the URL to the platform's opener. Only http(s) URLs reach it: the
// base is validated at `config set` and the path is built above.
func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

// validateAdminURL checks admin-url at `config set`: an http(s) URL with a host. Plain
// http is fine here — the CLI only hands it to a browser, it never sends a credential.
func validateAdminURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("admin-url must be an http(s) URL, e.g. https://rail0-admin-staging.fly.dev (got %q)", v)
	}
	return nil
}
