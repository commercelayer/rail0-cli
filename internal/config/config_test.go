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

// writeCorruptConfig plants a config file that YAML cannot parse — the state a
// stray character leaves behind, and the one the CLI could not recover from.
func writeCorruptConfig(t *testing.T) string {
	t.Helper()
	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("settings:\n  base-url: [unclosed\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// A reader must refuse an unparseable config — acting on settings we could not
// read is worse than stopping — and the message has to name the file and the way
// out, since it lives in an OS directory most people never open.
func TestLoadRefusesACorruptFileAndSaysWhereItIs(t *testing.T) {
	withTempConfigDir(t)
	path := writeCorruptConfig(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load must refuse a config it cannot parse")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error must name the file, got: %v", err)
	}
	if !strings.Contains(err.Error(), "rail0 config path") {
		t.Errorf("error must carry the remedy, got: %v", err)
	}
}

// ...but a WRITER must not, or the CLI has no way to carry out the advice that
// error just gave. Every config-writing command went through Load, so a single
// stray character disabled 'config set', 'config unset', 'env <name>',
// 'aliases add' and 'keys add' — all of which exist to rewrite that very file.
func TestLoadForWriteReplacesACorruptFileInsteadOfFailing(t *testing.T) {
	withTempConfigDir(t)
	writeCorruptConfig(t)

	cfg, replaced, err := LoadForWrite()
	if err != nil {
		t.Fatalf("LoadForWrite must not fail on a corrupt file: %v", err)
	}
	if !replaced {
		t.Error("replaced must be true so the caller can say the old contents are gone")
	}
	if cfg == nil || len(cfg.Settings) != 0 || len(cfg.Aliases) != 0 {
		t.Errorf("expected an empty config to build on, got %+v", cfg)
	}

	// And the write that follows must actually land — the point of the exercise.
	cfg.Settings = map[string]string{"base-url": "https://example.test"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save after replacing a corrupt config: %v", err)
	}
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("the rewritten file must parse: %v", err)
	}
	if got := reloaded.Settings["base-url"]; got != "https://example.test" {
		t.Errorf("base-url = %q, want the value just written", got)
	}
}

// On a healthy file LoadForWrite must be Load — same contents, and replaced
// false, so no command ever claims to have discarded settings it kept.
func TestLoadForWritePreservesAReadableFile(t *testing.T) {
	withTempConfigDir(t)
	original := &Config{
		Settings: map[string]string{"base-url": "https://kept.test"},
		Aliases:  map[string]string{"merchant": "0xabc"},
	}
	if err := original.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg, replaced, err := LoadForWrite()
	if err != nil {
		t.Fatalf("LoadForWrite: %v", err)
	}
	if replaced {
		t.Error("replaced must be false for a file that parsed")
	}
	if cfg.Settings["base-url"] != "https://kept.test" || cfg.Aliases["merchant"] != "0xabc" {
		t.Errorf("readable contents must survive, got %+v", cfg)
	}
}
