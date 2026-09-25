package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CheckInterval is how often the automatic check may ask GitHub for the latest
// release. Once a day keeps an interactive user current without a network round-trip
// on every command (and well inside GitHub's anonymous rate limit).
const CheckInterval = 24 * time.Hour

// checkState is what the automatic check remembers between runs.
type checkState struct {
	CheckedAt time.Time `json:"checked_at"`
}

// statePath is <user cache dir>/rail0/update-check.json, next to the session tokens.
func statePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rail0", "update-check.json"), nil
}

// CheckDue reports whether the automatic check should run now: never checked, the last
// check older than CheckInterval, or the state unreadable. A clock set backwards (last
// check in the future) also counts as due, so a bad timestamp cannot silence the check
// forever.
func CheckDue(now time.Time) bool {
	path, err := statePath()
	if err != nil {
		return false // nowhere to remember it: never check rather than check every run
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var st checkState
	if json.Unmarshal(raw, &st) != nil {
		return true
	}
	age := now.Sub(st.CheckedAt)
	return age < 0 || age >= CheckInterval
}

// MarkChecked records a check at now. Called whether or not the check succeeded, so a
// machine without network is asked once a day, not on every command. Best-effort: a
// failure to write only means the next command checks again.
func MarkChecked(now time.Time) {
	path, err := statePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	raw, err := json.Marshal(checkState{CheckedAt: now.UTC()})
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o600)
}
