// Package config manages the CLI's on-disk configuration: currently a global
// address book (name → wallet address) referenced as "@name" on address flags.
//
// The file lives in the OS config dir (config.yaml). It is optional — every
// command works without it. The JWT session token is NOT stored here; it stays
// in the cache dir (see the token package).
package config

import (
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
		return nil, err
	}
	return &c, nil
}

// Save writes the config back to disk (0600, dir 0700), creating the directory
// if needed.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}
