package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/rail0/cli/internal/config"
)

// TestMain is the same backstop internal/config carries, for the same reason:
// this package's tests write the config too (aliases add/rm, keys add/rm,
// env <name>, config set/unset all persist through config.Save), so a test
// whose isolation does not take overwrites the developer's real file. Every
// call site here currently redirects HOME alongside XDG_CONFIG_HOME, which is
// what makes os.UserConfigDir land in the temp dir on macOS; this check is what
// keeps that true for the tests not yet written.
//
// config.Path() is resolved BEFORE m.Run, while the environment is still the
// developer's own — afterwards the tests have redirected it and it would answer
// about a temp dir instead.
func TestMain(m *testing.M) {
	path, err := config.Path()
	if err != nil {
		path = ""
	}
	before, okBefore := realConfigFingerprint(path)

	code := m.Run()

	if after, okAfter := realConfigFingerprint(path); okBefore && okAfter && before != after {
		fmt.Fprintf(os.Stderr,
			"\nFAIL: the test run modified the real config file\n"+
				"      %s\n"+
				"      A test escaped its temp dir. Isolate it with isolate(t), which\n"+
				"      redirects HOME as well as XDG_CONFIG_HOME (os.UserConfigDir reads\n"+
				"      HOME on macOS and ignores XDG_CONFIG_HOME entirely).\n", path)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// realConfigFingerprint hashes the file at path, with a sentinel for "not
// there" so a test that creates a config where none existed is caught as well.
// The bool reports whether a comparable fingerprint could be taken at all.
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
