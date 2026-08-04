package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// init is a guided setup, so what is testable without a terminal is its contract: it
// exists, it takes no arguments, and it refuses rather than hanging when there is
// nobody to guide. The steps themselves are the existing commands (#105).
func TestInitCommandContract(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"init"})
	if err != nil || cmd.Name() != "init" {
		t.Fatalf("rail0 init is not registered (%v)", err)
	}
	if err := cmd.Args(cmd, []string{"staging"}); err == nil {
		t.Error("init takes no positional argument")
	}
}

// Stdin is not a terminal under `go test`, which is exactly the non-interactive case:
// the error must name the four commands to run instead, since that is the information
// a script needs.
func TestInitRefusesNonInteractively(t *testing.T) {
	err := initCmd.RunE(initCmd, nil)
	if err == nil {
		t.Fatal("init must refuse when stdin is not a terminal, not wait on a prompt")
	}
	for _, want := range []string{"rail0 env", "rail0 keys add", "rail0 auth login"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal should name %q; got: %v", want, err)
		}
	}
}

// The '@' is how a key is REFERENCED, not part of its name — `keys add @payer` is
// rejected downstream, so the form catches it at the point it is typed.
func TestValidateKeyName(t *testing.T) {
	if err := validateKeyName(""); err == nil {
		t.Error("an empty name must be rejected")
	}
	if err := validateKeyName("@payer"); err == nil {
		t.Error("a leading @ must be rejected")
	}
	if err := validateKeyName("payer"); err != nil {
		t.Errorf("a plain name was rejected: %v", err)
	}
}

// Sixteen commands listed alphabetically gave `completion` the same weight as
// `payments`. Every command a first-time reader meets carries a group; `completion`
// and `help` deliberately do not.
func TestTopLevelCommandsAreGrouped(t *testing.T) {
	// The meta commands stay under "Additional Commands" on purpose.
	ungrouped := map[string]bool{"completion": true, "help": true, "version": true}
	for _, c := range rootCmd.Commands() {
		if ungrouped[c.Name()] {
			if c.GroupID != "" {
				t.Errorf("%s should stay ungrouped (Additional Commands)", c.Name())
			}
			continue
		}
		if c.GroupID == "" {
			t.Errorf("%s has no help group — it would land under Additional Commands", c.Name())
		}
	}
	if len(rootCmd.Groups()) == 0 {
		t.Error("no help groups are registered")
	}
}

// The pair that reads as a typo either way round: `token` is the session JWT, `tokens`
// is the payment-token catalog. The distinction was explained only inside
// cmd/token.go, so the root help now carries it.
func TestRootHelpDisambiguatesTokenFromTokens(t *testing.T) {
	long := rootCmd.Long
	if !strings.Contains(long, "rail0 token") || !strings.Contains(long, "rail0 tokens") {
		t.Error("the root help should name both 'rail0 token' and 'rail0 tokens'")
	}
	if !strings.Contains(long, "QUICKSTART") {
		t.Error("the root help should open with a quickstart")
	}
}

var _ = cobra.Command{}
