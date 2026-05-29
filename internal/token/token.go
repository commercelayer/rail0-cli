// Package token manages the JWT session token for the CLI.
//
// Priority: RAIL0_TOKEN env var > ~/.cache/rail0/token file.
// The token file is written by `rail0 auth login` and read by any command
// that needs authentication.
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

// Load returns the stored JWT, or "" if none is found.
// Priority: RAIL0_TOKEN env var > cache file.
func Load() string {
	if t := os.Getenv("RAIL0_TOKEN"); t != "" {
		return strings.TrimSpace(t)
	}
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
