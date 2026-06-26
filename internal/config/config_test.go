package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempConfigDir points os.UserConfigDir at a temp dir for the duration of a
// test by setting the platform's config-dir env var, then restores it.
func withTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	// os.UserConfigDir reads XDG_CONFIG_HOME on Linux and HOME on macOS.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	withTempConfigDir(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if cfg == nil || len(cfg.Aliases) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestSaveAndReload(t *testing.T) {
	withTempConfigDir(t)
	cfg := &Config{Aliases: map[string]string{"payee": "0xabc"}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path, _ := Path()
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("unexpected path %s", path)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 file, got mode=%v err=%v", info.Mode().Perm(), err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Aliases["payee"] != "0xabc" {
		t.Errorf("alias roundtrip mismatch: %+v", got.Aliases)
	}
}
