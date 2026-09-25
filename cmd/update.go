package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/rail0/cli/internal/selfupdate"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var updateCheckOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rail0 to the latest release",
	Long: `Download the latest rail0 release and install it in place of this binary.

The archive for this platform is downloaded from the GitHub release (with a progress
bar on a terminal), its SHA-256 is verified against the release's checksums.txt, and
the binary inside replaces the running one. Nothing on disk changes unless the
download verifies. The r0 alias follows, since it is a link to the same file.

If rail0 lives in a directory you cannot write to (e.g. /usr/local/bin, where
'make install' puts it), run it with sudo.

Installed with Homebrew, rail0 never replaces itself: it tells you to run
'brew upgrade rail0', so Homebrew keeps track of the installed version.

Interactive sessions also check for a new release once a day and offer to install
it. The check never runs in CI, with --json or --quiet, when stdin or stderr is not
a terminal, or with RAIL0_NO_UPDATE_CHECK=1.`,
	Example: `  rail0 update           # install the latest release
  rail0 update --check   # only report whether one is available`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		up := selfupdate.New(&http.Client{})
		ctx := cmd.Context()

		lookup, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		rel, err := up.Latest(lookup)
		if err != nil {
			return err
		}
		selfupdate.MarkChecked(time.Now())

		if !selfupdate.Updatable(version) {
			if updateCheckOnly {
				fmt.Printf("Latest release: %s (this is a development build, %s)\n", rel.Version, version)
				return nil
			}
			return fmt.Errorf("this is a development build (%s), not a release; install a release binary to use 'rail0 update' (latest: %s)", version, rel.Version)
		}
		if !selfupdate.Newer(version, rel.Version) {
			fmt.Printf("rail0 %s is the latest version\n", version)
			return nil
		}
		// Installed by a package manager: the upgrade goes through it, never around it
		// (see selfupdate.ManagedBy). Not an error — there is a newer release, and this is
		// how to get it.
		manager := installedBy()
		if updateCheckOnly || manager != "" {
			fmt.Printf("rail0 %s is available (installed: %s). Run '%s' to install it.\n",
				rel.Version, version, selfupdate.UpgradeCommand(manager))
			return nil
		}
		if err := installRelease(ctx, up, rel); err != nil {
			return err
		}
		fmt.Printf("rail0 updated: %s → %s\n", version, rel.Version)
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "Only report whether a newer release is available")
	rootCmd.AddCommand(updateCmd)
}

// installedBy is the package manager that owns the running binary, or "" (see
// selfupdate.ManagedBy). An unresolvable path counts as unmanaged: the install step then
// reports the real error.
func installedBy() string {
	exe, err := selfupdate.Executable()
	if err != nil {
		return ""
	}
	return selfupdate.ManagedBy(exe)
}

// installRelease downloads rel and swaps it in place of the running binary, drawing a
// progress bar on stderr when it is a terminal and a plain line otherwise.
func installRelease(ctx context.Context, up *selfupdate.Updater, rel *selfupdate.Release) error {
	exe, err := selfupdate.Executable()
	if err != nil {
		return fmt.Errorf("locating the rail0 binary: %w", err)
	}
	label := fmt.Sprintf("Downloading rail0 %s", rel.Version)
	var progress selfupdate.Progress
	var bar *ui.ProgressBar
	if term.IsTerminal(os.Stderr.Fd()) {
		bar = ui.NewProgressBar(os.Stderr, label)
		progress = bar.Update
	} else {
		fmt.Fprintln(os.Stderr, label+"…")
	}
	err = up.Install(ctx, rel, exe, progress)
	if bar != nil {
		bar.Done()
	}
	if errors.Is(err, selfupdate.ErrNotWritable) {
		return fmt.Errorf("%w\n\nRun it with elevated permissions instead: sudo rail0 update", err)
	}
	return err
}

// updateCheckSkipped lists the commands the automatic check never interrupts: the
// update command itself, the meta commands, and shell completion (which runs on every
// Tab press and must answer instantly and silently).
var updateCheckSkipped = map[string]bool{
	"update": true, "version": true, "help": true, "completion": true,
	cobra.ShellCompRequestCmd: true, cobra.ShellCompNoDescRequestCmd: true,
}

// maybeOfferUpdate is the automatic update check, run before every command.
//
// It asks GitHub for the latest release at most once per selfupdate.CheckInterval
// and, when a newer one exists, offers to install it (default yes). Accepting
// downloads it with a progress bar and swaps the binary; the command the user typed
// then runs to completion on the already-loaded old code, so nothing they asked for
// is lost or changed mid-way.
//
// It is deliberately timid, because it runs in front of real work:
//   - only for an interactive human: stdin and stderr are terminals, no --json or
//     --quiet, not in CI (the CI variable every major provider sets), and not
//     opted out with RAIL0_NO_UPDATE_CHECK — a script must never block on a prompt
//     or find a different binary between two steps;
//   - never for a local build (dev / git describe), which has nothing to compare;
//   - the lookup has a short timeout, and ANY failure — offline, rate-limited, no
//     release yet — is silent and simply waits for the next day's check;
//   - a failed install warns and lets the command carry on.
func maybeOfferUpdate(cmd *cobra.Command) {
	if updateCheckSkipped[topLevelName(cmd)] || !updateCheckWanted() {
		return
	}
	now := time.Now()
	if !selfupdate.CheckDue(now) {
		return
	}
	// Marked before the lookup, so a check that fails (or is interrupted) still counts
	// and a machine without network is not asked on every command.
	selfupdate.MarkChecked(now)

	up := selfupdate.New(&http.Client{})
	lookup, cancel := context.WithTimeout(cmd.Context(), 3*time.Second)
	defer cancel()
	rel, err := up.Latest(lookup)
	if err != nil || !selfupdate.Newer(version, rel.Version) {
		return
	}

	// A package-managed install is told how to upgrade, not offered an in-place swap.
	if manager := installedBy(); manager != "" {
		fmt.Fprintf(os.Stderr, "A new rail0 release is available: %s → %s. Run '%s' to install it.\n\n",
			version, rel.Version, selfupdate.UpgradeCommand(manager))
		return
	}
	if !promptYes(os.Stdin, os.Stderr, fmt.Sprintf("A new rail0 release is available: %s → %s. Update now?", version, rel.Version)) {
		fmt.Fprintln(os.Stderr, "Skipped. Run 'rail0 update' whenever you want it.")
		return
	}
	if err := installRelease(cmd.Context(), up, rel); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\nContinuing with the installed version; run 'rail0 update' to retry.\n", ui.ErrStyle.Render("Update failed:"), err)
		return
	}
	fmt.Fprintf(os.Stderr, "rail0 updated to %s. Continuing with your command…\n\n", rel.Version)
}

// updateCheckWanted reports whether this session is one the automatic check may
// interrupt — see maybeOfferUpdate.
func updateCheckWanted() bool {
	switch {
	case !selfupdate.Updatable(version),
		jsonOut, quietOut,
		os.Getenv("CI") != "",
		os.Getenv("RAIL0_NO_UPDATE_CHECK") != "",
		!term.IsTerminal(os.Stdin.Fd()),
		!term.IsTerminal(os.Stderr.Fd()):
		return false
	}
	return true
}

// promptYes asks a [Y/n] question: Enter or anything starting with "y" accepts,
// anything else (including EOF) declines.
func promptYes(in io.Reader, out io.Writer, question string) bool {
	fmt.Fprintf(out, "%s [Y/n]: ", question)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(out)
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || strings.HasPrefix(answer, "y")
}
