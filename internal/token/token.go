// Package token manages the JWT session token for the CLI.
//
// The token lives in exactly ONE place: ~/.cache/rail0/token, written by
// `rail0 auth login` and read by any command that needs authentication.
//
// There is deliberately no environment-variable override. A RAIL0_TOKEN env var used
// to take priority over the file, which made `auth login` look broken: it wrote a
// fresh token to disk, reported success, and every subsequent command kept using the
// stale env value — so a user with an expired RAIL0_TOKEN exported in their shell
// re-logged-in endlessly with no effect and no explanation (#60). A single source
// removes the shadowing entirely rather than reordering it, and keeps a bearer
// credential out of the environment, where it leaks into child processes and `env`
// dumps.
package token

import (
	"os"
	"path/filepath"
	"strings"
)

// tokenFilePath returns the path to the cached token file.
func tokenFilePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rail0", "token"), nil
}

// Load returns the stored JWT, or "" if none is found. The cache file is the only
// source — see the package comment for why there is no env override.
func Load() string {
	path, err := tokenFilePath()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Save writes the JWT to the cache file.
func Save(jwt string) error {
	_, err := SaveAndReport(jwt)
	return err
}

// SaveAndReport writes the JWT and returns the file path on success.
func SaveAndReport(jwt string) (string, error) {
	path, err := tokenFilePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(jwt+"\n"), 0600); err != nil {
		return "", err
	}
	return path, nil
}

// Delete removes the cached token file.
func Delete() error {
	path, err := tokenFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
