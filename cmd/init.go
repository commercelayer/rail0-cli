package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// `rail0 init` — the guided first-run setup.
//
// The path from a downloaded binary to a first payment was eight to ten steps the
// reader had to discover one at a time: pick a gateway, learn that a session is
// per-gateway, save a key, learn the @name convention, sign in, check the gateway is
// actually reachable. Each is one command, and each is obvious ONLY once you know the
// model. (#105)
//
// It composes the existing commands rather than reimplementing them — switchEnv,
// keysAddCmd, authLoginCmd, Health.Get — so there is exactly one behaviour per step
// and `keys add`'s refusal to overwrite an orphaned keychain entry (#70) still applies
// here. Every step can be skipped, so a partly-configured machine can run it again.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Guided first-run setup: environment, key, sign-in",
	Long: `Walk through the setup a first payment needs, in one command:

  1. choose an environment (writes base-url, so every later command uses it)
  2. check the gateway answers
  3. optionally save a signing key in the OS keychain (referenced later as @name)
  4. optionally sign in with it (SIWE), caching a session for that gateway

Each step can be skipped, so it is safe to re-run on a machine that is already
half-configured. Nothing here is irreversible: the same steps are 'rail0 env',
'rail0 health', 'rail0 keys add' and 'rail0 auth login', and the setup can be
undone with 'rail0 keys rm' / 'rail0 auth logout'.

Interactive by design — it asks questions. In a script, run those four commands
directly instead.`,
	Example: `  rail0 init`,
	Args:    cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		// A guided setup with nobody to guide is a hang waiting on stdin, so it fails
		// with the commands it would have run instead — which is the information a
		// script needs anyway.
		if !interactive() {
			return newUsageError("interactive_only",
				"'rail0 init' is interactive; in a script use: rail0 env <name> && rail0 health && "+
					"rail0 keys add <name> && rail0 auth login -p @<name>")
		}

		ctx := context.Background()

		// ── 1. Environment ────────────────────────────────────────────────────
		names := make([]huh.Option[string], 0, len(environments))
		for _, e := range environments {
			names = append(names, huh.NewOption(fmt.Sprintf("%s — %s", e.name, e.settings["base-url"]), e.name))
		}
		env := detectEnv()
		if env == "custom" {
			env = environments[0].name
		}
		if err := huh.NewSelect[string]().
			Title("Which gateway?").
			Description("Written to the config file as base-url; --base-url still overrides it per command.").
			Options(names...).
			Value(&env).
			Run(); err != nil {
			return initPromptErr(err)
		}
		if err := switchEnv(env, false); err != nil {
			return err
		}

		// ── 2. Does it answer? ────────────────────────────────────────────────
		// Before the key and the sign-in, because both are wasted effort against a
		// gateway that is not there — and "connection refused" after pasting a
		// private key reads as if the key were the problem.
		fmt.Fprintf(os.Stderr, "\nChecking %s…\n", resolveBaseURL())
		if _, err := newReadClient().Health.Get(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: the gateway did not answer — %s\n", humanizeError(err))
			fmt.Fprintln(os.Stderr, "Setup continues; run 'rail0 health' once it is up.")
		} else {
			fmt.Fprintln(os.Stderr, "Gateway is up.")
		}

		// ── 3. Signing key ────────────────────────────────────────────────────
		saveKey := false
		if err := huh.NewConfirm().
			Title("Save a signing key in the OS keychain?").
			Description("Stored by the OS, never in a file. Referenced later as @name — no key on the command line.").
			Value(&saveKey).
			Run(); err != nil {
			return initPromptErr(err)
		}

		keyName := ""
		if saveKey {
			if err := huh.NewInput().
				Title("Name for the key").
				Placeholder("payer").
				Value(&keyName).
				Validate(validateKeyName).
				Run(); err != nil {
				return initPromptErr(err)
			}
			// The real command, guards included — notably its refusal to overwrite an
			// orphaned keychain entry, which is the one mistake here that cannot be
			// undone.
			if err := keysAddCmd.RunE(keysAddCmd, []string{keyName}); err != nil {
				return err
			}
		}

		// ── 4. Sign in ────────────────────────────────────────────────────────
		if keyName != "" {
			signIn := true
			if err := huh.NewConfirm().
				Title(fmt.Sprintf("Sign in now with @%s?", keyName)).
				Description("SIWE sign-in; the session is cached for this gateway only.").
				Value(&signIn).
				Run(); err != nil {
				return initPromptErr(err)
			}
			if signIn {
				// authLoginCmd reads the key from its flag variable, so this is the
				// same code path as `rail0 auth login -p @name`.
				loginPrivateKey = "@" + keyName
				if err := authLoginCmd.RunE(authLoginCmd, nil); err != nil {
					return err
				}
			}
		}

		printNextSteps(keyName)
		return nil
	},
}

// initPromptErr maps a cancelled init prompt (Ctrl-C or ESC: huh.ErrUserAborted) onto
// ErrAborted, so `rail0 init` exits 130 like every other prompt and as the README
// documents, instead of 1 — a script must be able to tell "the user said no" from a
// failure. Returned rather than os.Exit'd (abortOrReturn): init's prompts are steps of
// one command, so ending through RunE keeps the single exit path. Any other error is
// returned unchanged.
func initPromptErr(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Fprintln(os.Stderr)
		return ErrAborted
	}
	return err
}

// validateKeyName rejects a name the keychain path would reject later, at the point
// it is typed rather than after the secret has been pasted.
func validateKeyName(name string) error {
	if name == "" {
		return errors.New("a name is required")
	}
	if name[0] == '@' {
		return errors.New("leave out the leading '@' — it is how you REFERENCE the key, not part of its name")
	}
	return nil
}

// printNextSteps closes the setup with the commands that follow from it, which is the
// part a first-time reader cannot infer: the discovery call is public, and creating a
// payment needs the payer to be the signed-in address.
//
// It leads with 'aliases add' and then uses @names throughout, including the key it
// just saved. Setup is where the habit is cheapest to form, and a first example that
// pastes raw hex teaches the opposite of everything downstream.
func printNextSteps(keyName string) {
	ref := "@<name>"
	if keyName != "" {
		ref = "@" + keyName
	}
	fmt.Fprintf(os.Stderr, `
Set up. From here:

  rail0 aliases add merchant 0x…      # name the addresses once, then use @merchant
  rail0 payment-methods -A @merchant  # what that merchant accepts (public, no login)
  rail0 payments create -F @me -T @merchant -t USDC -a 1.00 -c 5042002 -p %s
  rail0 payments list                 # your payments, as payer or payee

'rail0 <command> --help' has the detail; 'rail0 env current -d' shows what this wrote.
`, ref)
}

func init() {
	rootCmd.AddCommand(initCmd)
}
