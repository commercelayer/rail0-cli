package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var assumeYes bool

// addYesFlag registers -y/--yes on a destructive command so its confirmation
// prompt can be skipped (and so the op can run non-interactively).
func addYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Skip the confirmation prompt (required to run non-interactively)")
}

// confirmDestructive gates an irreversible action behind a y/N prompt. With --yes
// it passes silently. In a non-interactive shell (no TTY) it refuses unless --yes
// was given — so a script never blocks on a hidden prompt nor runs a destructive
// op by accident. Returns an error (aborting the command) when not confirmed.
func confirmDestructive(prompt string) error {
	if assumeYes {
		return nil
	}
	if !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("refusing to run without confirmation in a non-interactive shell; pass --yes to proceed")
	}
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		// ErrAborted so declining a confirmation exits with the same code as
		// cancelling a prompt — one story for "the user said no". (#65)
		return ErrAborted
	}
}
