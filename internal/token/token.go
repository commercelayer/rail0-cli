// Package token manages the JWT session tokens for the CLI.
//
// Tokens are stored PER GATEWAY, under ~/.cache/rail0/tokens/<digest>, where the
// digest is derived from the gateway's base URL. A JWT is minted by one gateway
// and is meaningless to another, and `rail0 env` switches base URL without
// touching the session — so a single global file made this happen: log in on
// staging, `rail0 env production`, and the next command found a live token whose
// `sub` matched, skipped the sign-in it exists to perform, and got a 401 from
// production. Keying by gateway both fixes that and keeps each environment's
// session alive across switches. (#94)
//
// There is deliberately no environment-variable override. A RAIL0_TOKEN env var
// used to take priority over the file, which made `auth login` look broken: it
// wrote a fresh token to disk, reported success, and every subsequent command
// kept using the stale env value — so a user with an expired RAIL0_TOKEN exported
// in their shell re-logged-in endlessly with no effect and no explanation (#60).
// A single source removes the shadowing entirely rather than reordering it, and
// keeps a bearer credential out of the environment, where it leaks into child
// processes and `env` dumps.
package token

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/rail0/cli/internal/ui"
)

// legacyFileName is the pre-#94 single-token file, which carried no record of
// which gateway issued it. It is never READ (its origin is unknowable), only
// cleaned up — see Load.
const legacyFileName = "token"

// tokenDir returns ~/.cache/rail0.
func tokenDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rail0"), nil
}

// tokenFilePath returns the token path for a gateway base URL.
//
// The file name is a digest rather than the URL itself: a base URL contains
// characters that are awkward or unsafe in a path (scheme separators, ports,
// sub-paths), and the digest keeps one flat, predictable directory. Normalized
// first, so "https://api.rail0.xyz" and "https://api.rail0.xyz/" are one session.
func tokenFilePath(baseURL string) (string, error) {
	dir, err := tokenDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(NormalizeBaseURL(baseURL)))
	return filepath.Join(dir, "tokens", hex.EncodeToString(sum[:8])), nil
}

// NormalizeBaseURL is the one notion of "the same gateway": case, surrounding
// space and trailing slashes do not change which session a URL names. Exported so
// every comparison of base URLs agrees with the one the session store makes.
func NormalizeBaseURL(baseURL string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
}

// Path returns where this gateway's session is (or would be) stored, or "" if the
// cache directory cannot be resolved.
//
// Reported by `auth status` because the location is per-gateway and digest-named
// (see tokenFilePath): the operator cannot derive it from the documentation, so
// without this there is no way to inspect or remove one specific session by hand.
// It does not check whether the file exists — the caller already knows, and a path
// for a gateway with no session is still the right answer to "where would it be?".
func Path(baseURL string) string {
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return ""
	}
	return path
}

// Load returns the stored JWT for this gateway, or "" if none is found.
//
// A leftover pre-#94 token file is removed, with a one-line notice, rather than
// used: nothing recorded which gateway minted it, so trusting it is exactly the
// bug this layout fixes. The cost is one re-login after upgrading, said out loud
// instead of surfacing later as an unexplained 401.
func Load(baseURL string) string {
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return ""
	}
	if b, readErr := os.ReadFile(path); readErr == nil {
		return strings.TrimSpace(string(b))
	}

	if dir, dirErr := tokenDir(); dirErr == nil {
		legacy := filepath.Join(dir, legacyFileName)
		if _, statErr := os.Stat(legacy); statErr == nil {
			_ = os.Remove(legacy)
			ui.Notice("note", map[string]any{"what": "legacy_session_removed"},
				"note: sessions are now stored per gateway — run 'rail0 auth login' once more for this one")
		}
	}
	return ""
}

// Save writes the JWT for this gateway.
func Save(baseURL, jwt string) error {
	_, err := SaveAndReport(baseURL, jwt)
	return err
}

// SaveAndReport writes the JWT and returns the file path on success.
//
// Permissions are ENFORCED, not requested. MkdirAll and WriteFile apply their mode
// only when they create the path, so a tokens directory or file that already
// existed as 0755/0644 (an older build, a restored backup, a umask) stayed readable
// by every local user, holding a bearer credential. The directory is chmod'ed on
// every save, and the file is replaced rather than rewritten.
//
// Replaced ATOMICALLY: written to a 0600 temp file beside it and renamed over it, so
// a command reading the session while another saves one sees the old token or the
// new one — never a truncated file, which parses as no session and forces a login.
// Same directory because os.Rename is only atomic within a filesystem.
func SaveAndReport(baseURL, jwt string) (string, error) {
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return "", err
	}
	return path, writeTokenFile(path, jwt)
}

// writeTokenFile is the one way a token reaches the disk: the session's and the
// write-mode one alike, with the permissions and the atomic replace described on
// SaveAndReport.
func writeTokenFile(path, jwt string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".token-*.tmp") // created 0600
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // a no-op once renamed
	if _, err := tmp.WriteString(jwt + "\n"); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Delete removes this gateway's cached token (and any legacy leftover).
func Delete(baseURL string) error {
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if dir, dirErr := tokenDir(); dirErr == nil {
		_ = os.Remove(filepath.Join(dir, legacyFileName))
	}
	_ = os.Remove(path + operatorSuffix) // what was known about the session goes with it
	return nil
}

// ── Operator marker ──────────────────────────────────────────────────────────
//
// Whether the stored session belongs to a gateway operator. The session token does not
// say (the gateway reads the grant on every request), so the answer the gateway gave at
// sign-in is kept beside the token, in "<digest>.operator".
//
// The marker holds a fingerprint of the token it describes, not a bare yes: a marker left
// behind by an earlier session says nothing about the one stored now, so it can never be
// read as an answer about another token.

const operatorSuffix = ".operator"

func fingerprint(jwt string) string {
	sum := sha256.Sum256([]byte(jwt))
	return hex.EncodeToString(sum[:])
}

// MarkOperator records that jwt, this gateway's session, is an operator's.
func MarkOperator(baseURL, jwt string) error {
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return err
	}
	return writeTokenFile(path+operatorSuffix, fingerprint(jwt))
}

// UnmarkOperator removes the marker. No marker is not an error.
func UnmarkOperator(baseURL string) error {
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return err
	}
	if err := os.Remove(path + operatorSuffix); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsOperator reports whether the session stored for this gateway was an operator's when
// it signed in. False when there is no session, no marker, or a marker of another token.
// A session stored by a version that kept no marker, or pasted in as a raw token, reads
// false: nothing was asked about it.
func IsOperator(baseURL string) bool {
	jwt := Load(baseURL)
	if jwt == "" {
		return false
	}
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(path + operatorSuffix)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == fingerprint(jwt)
}

// ── Write-mode token ─────────────────────────────────────────────────────────
//
// An operator session reads; to write it asks the gateway for a second token, valid
// for an hour ('rail0 admin write-mode on'). That token lives beside the session's, in
// "<digest>.write": per gateway like the session, and a file of its own so that ending
// write mode is removing it, with the session untouched.

// writeModeFilePath is where this gateway's write-mode token is (or would be) stored.
func writeModeFilePath(baseURL string) (string, error) {
	path, err := tokenFilePath(baseURL)
	if err != nil {
		return "", err
	}
	return path + ".write", nil
}

// LoadWriteMode returns this gateway's stored write-mode token, or "" if there is none.
// It does not judge the token: whether it is still valid, and whether it belongs to the
// stored session, is the caller's to check.
func LoadWriteMode(baseURL string) string {
	path, err := writeModeFilePath(baseURL)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SaveWriteMode stores this gateway's write-mode token, with the session file's
// permissions and atomic replace.
func SaveWriteMode(baseURL, jwt string) error {
	path, err := writeModeFilePath(baseURL)
	if err != nil {
		return err
	}
	return writeTokenFile(path, jwt)
}

// DeleteWriteMode removes this gateway's write-mode token. No token is not an error.
func DeleteWriteMode(baseURL string) error {
	path, err := writeModeFilePath(baseURL)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
