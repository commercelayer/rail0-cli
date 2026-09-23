package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// realConfigFingerprint hashes the developer's ACTUAL config file, resolved
// once before any test has had a chance to redirect HOME or XDG_CONFIG_HOME.
// A missing file gets its own sentinel rather than an empty hash, so a test
// that CREATES one where there was none is caught too. The bool reports only
// whether the path could be resolved at all; an unreadable file is not a
// finding, it is simply not a fingerprint we can compare.
func realConfigFingerprint(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "absent", true
	}
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), true
}

// TestMain is the backstop for the failure this package shipped twice: a test
// whose isolation did not take, silently writing its fixture over the real
// ~/Library/Application Support/rail0/config.yaml. withTempConfigDir now
// asserts its own redirection, but that only protects tests that CALL it — and
// both offenders had hand-rolled their setup instead. This check does not care
// how a test isolates, or whether it isolates at all. It compares the file on
// disk before and after the whole package runs, so any future test that
// reaches the real config fails the run no matter how it got there.
//
// It deliberately cannot say WHICH test did it: that costs a `go test -run`
// bisect, which is a small price for a guarantee that has no holes. The config
// file is the only copy of the address aliases (key secrets live in the OS
// keychain), so losing it is unrecoverable and worth over-protecting.
func TestMain(m *testing.M) {
	var path string
	if dir, err := os.UserConfigDir(); err == nil {
		path = filepath.Join(dir, "rail0", "config.yaml")
	}
	before, okBefore := realConfigFingerprint(path)

	code := m.Run()

	if after, okAfter := realConfigFingerprint(path); okBefore && okAfter && before != after {
		fmt.Fprintf(os.Stderr,
			"\nFAIL: the test run modified the real config file\n"+
				"      %s\n"+
				"      A test escaped its temp dir. Isolate it with withTempConfigDir(t),\n"+
				"      which redirects HOME as well as XDG_CONFIG_HOME (os.UserConfigDir\n"+
				"      reads HOME on macOS and ignores XDG_CONFIG_HOME entirely).\n", path)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
