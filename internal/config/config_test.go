package config

import (
	"os"
	"path/filepath"
	"strings"
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

// ── atomic Save + actionable Load errors (#108) ──────────────────────────────

// Save must not be able to leave a truncated file behind: `Keys` is the only
// index of what lives in the OS keychain, so a half-written config turns saved
// signing keys into orphans — still in the keychain, no longer referenceable by
// name. The observable guarantee is that the write goes through a temp file in
// the SAME directory (os.Rename is only atomic within one filesystem) and leaves
// no litter.
func TestSaveIsAtomicAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := Path(); err != nil {
		t.Skipf("no config dir on this platform: %v", err)
	}

	first := &Config{Keys: []string{"payer"}, Settings: map[string]string{"base-url": "http://a"}}
	if err := first.Save(); err != nil {
		t.Fatal(err)
	}
	// Overwrite, which is the interesting case: the previous file must be
	// replaced wholesale, never opened for truncation in place.
	second := &Config{Keys: []string{"payer", "payee"}}
	if err := second.Save(); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Keys) != 2 {
		t.Errorf("Keys = %v, want both entries", got.Keys)
	}

	path, _ := Path()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// A malformed file must say WHICH file and what to do about it: the bare
// `yaml: line 4: ...` left the reader hunting for a path in an OS-specific
// directory they never look at.
func TestLoadNamesThePathAndTheRemedyOnMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := Path()
	if err != nil {
		t.Skipf("no config dir on this platform: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("settings:\n  base-url: [unclosed\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("a malformed config must be an error, not silently empty")
	}
	for _, want := range []string{path, "rail0 config path"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err.Error(), want)
		}
	}
}
