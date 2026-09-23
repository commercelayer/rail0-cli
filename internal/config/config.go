// Package config manages the CLI's on-disk configuration: currently a global
// address book (name → wallet address) referenced as "@name" on address flags.
//
// The file lives in the OS config dir (config.yaml). It is optional — every
// command works without it. The JWT session token is NOT stored here; it stays
// in the cache dir (see the token package).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultBaseURL is the production RAIL0 gateway endpoint used when neither
// --base-url nor RAIL0_BASE_URL is set.
const DefaultBaseURL = "https://api.rail0.xyz"

// Config is the on-disk CLI configuration.
type Config struct {
	// Aliases is the address book: name → 0x address, referenced as "@name" on
	// address flags (--from / --to).
	Aliases map[string]string `yaml:"aliases,omitempty"`
	// Keys records the private-key names saved in the OS keychain, referenced as
	// "@name" on the --private-key flag. Only the NAMES live here — the secrets
	// stay in the keychain (CLAUDE.md rule 8).
	//
	// It is a HINT FOR SHELL COMPLETION, not an inventory, and nothing presents
	// it as one. A keychain cannot be enumerated, so this list is a copy that
	// the keychain never confirms: delete this file, or carry it to another
	// machine, and it names keys that are not there, or misses keys that are.
	// `keys` therefore has no listing — `keys address <name>` asks the keychain
	// itself — and the only cost of a stale entry here is a wrong suggestion at
	// the TAB key. Do not reintroduce a reader that treats it as truth. (#139)
	Keys []string `yaml:"keys,omitempty"`
	// Settings holds persisted CLI defaults (e.g. "base-url"), managed by the
	// `config` command. Resolution precedence in the commands is
	// flag > env > this file > built-in default. Only non-sensitive values are
	// ever stored here — never private keys or tokens (CLAUDE.md rule 8).
	Settings map[string]string `yaml:"settings,omitempty"`
}

// Path returns the config file path (config.yaml in the OS config dir).
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rail0", "config.yaml"), nil
}

// Load reads the config file. A missing file is not an error — it returns an
// empty (non-nil) Config so callers treat "no config" and "empty config" alike.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		// Named, with the remedy: a bare `yaml: line 4: ...` left the reader
		// hunting for WHICH file, and this one is in an OS-specific directory
		// most people never look at. (#108)
		return nil, &notYAMLError{fmt.Errorf(
			"config file is not valid YAML (%s): %w\n"+
				"       run 'rail0 config path' to locate it, then fix or delete it "+
				"(the CLI works fine with no config file)",
			path, err)}
	}
	return &c, nil
}

// notYAMLError marks a Load failure caused by the file's CONTENT, as opposed to
// reading it. The distinction is what LoadForWrite rests on: content YAML cannot
// parse is unrecoverable and may be replaced, but a file that could not be READ
// (permissions, an I/O error) may be perfectly healthy and must never be.
type notYAMLError struct{ err error }

func (e *notYAMLError) Error() string { return e.err.Error() }
func (e *notYAMLError) Unwrap() error { return e.err }

// Save writes the config back to disk (0600, dir 0700), creating the directory
// if needed.
//
// ATOMIC: written to a temp file in the same directory and renamed over the
// target, so an interrupted write (a crash, a full disk, a Ctrl-C) leaves the
// previous file intact instead of a truncated one. That matters more here than
// the size of the file suggests — `Keys` is the ONLY index of what is stored in
// the OS keychain (the backend cannot enumerate entries), so a truncated config
// turns saved signing keys into orphans: still in the keychain, no longer
// referenceable by name. (#108)
//
// Same directory on purpose: os.Rename is only atomic within a filesystem, and
// the OS temp dir can be on another one.
// LoadForWrite reads the config for a caller that is about to REWRITE it. It is
// Load, except that an unparseable file yields an empty Config and replaced=true
// instead of an error.
//
// Reading and writing want opposite things from a corrupt file. A reader must not
// act on settings it cannot parse, so Load refuses — correctly. But when every
// WRITER refused as well, the CLI had no way out of the state its own error
// message tells you to leave: "fix or delete it" was advice that no command could
// carry out. `config set`, `config unset`, `env <name>`, `aliases add` and `keys
// add` all failed on precisely the file they exist to rewrite, so a single stray
// character disabled the whole local-configuration surface until the user found
// the file by hand.
//
// Discarding it costs nothing that was not already lost: keys inside a file YAML
// cannot parse are unrecoverable by any means, so there is nothing to preserve.
// The caller's only remaining job is to SAY that the old contents went away.
func LoadForWrite() (cfg *Config, replaced bool, err error) {
	c, err := Load()
	if err == nil {
		return c, false, nil
	}
	// Only a PARSE failure may be replaced. A read failure — an unreadable file, a
	// transient I/O error, no config dir — says nothing about the contents, and
	// treating it as "corrupt" rewrote a healthy file with only the new entry,
	// losing every alias, setting and the keychain index with it.
	var parseErr *notYAMLError
	if !errors.As(err, &parseErr) {
		return nil, false, err
	}
	return &Config{}, true, nil
}

func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup: a successful rename makes this a no-op, a failed one
	// leaves no litter behind.
	defer func() { _ = os.Remove(tmpName) }()

	// 0600 before any content is written: CreateTemp makes it 0600 already, but
	// being explicit keeps the guarantee visible next to the write.
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	// Flushed to the device before the rename: without it a crash right after
	// can leave the renamed file present but empty, which is the very outcome
	// the rename is meant to prevent.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
