package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Below the root, cobra prints a group's help and exits 0 on an argument it does not
// know, so `rail0 webhooks lsit` "succeeded". A stray argument must now fail — with
// a suggestion — while the bare group still just prints its help.
func TestGroupCommandsRejectUnknownSubcommands(t *testing.T) {
	root := &cobra.Command{Use: "rail0"}
	group := &cobra.Command{Use: "webhooks"}
	group.AddCommand(&cobra.Command{Use: "list", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(group)
	rejectUnknownSubcommands(root)

	root.SetArgs([]string{"webhooks", "lsit"})
	root.SetOut(new(strings.Builder))
	root.SetErr(new(strings.Builder))
	err := root.Execute()
	if err == nil {
		t.Fatal("an unknown subcommand must be an error, not help with exit 0")
	}
	if !strings.Contains(err.Error(), `unknown command "lsit"`) || !strings.Contains(err.Error(), "list") {
		t.Errorf("error should name the typo and suggest 'list', got: %v", err)
	}

	root.SetArgs([]string{"webhooks"})
	if err := root.Execute(); err != nil {
		t.Errorf("a bare group must still print its help without an error: %v", err)
	}
}
