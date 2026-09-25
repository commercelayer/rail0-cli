package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"1.3.0", "1.4.0", true},
		{"v1.3.0", "1.3.1", true},
		{"1.3.0", "2.0.0", true},
		{"1.9.0", "1.10.0", true}, // numeric, not lexical
		{"1.4.0", "1.4.0", false},
		{"1.4.1", "1.4.0", false},
		{"1.4.0-rc.1", "1.4.0", true}, // a release beats its own prerelease
		{"1.4.0", "1.4.0-rc.1", false},
		// Local builds are never offered an update over themselves.
		{"dev", "1.4.0", false},
		{"abc1234", "1.4.0", false},
		{"v1.3.0-5-gabc1234", "1.3.0", false},
		{"v1.3.0-5-gabc1234-dirty", "1.4.0", false},
		{"v1.3.0-dirty", "1.4.0", false},
		{"1.3.0", "garbage", false},
	}
	for _, c := range cases {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestArchiveNameFollowsGoReleaser(t *testing.T) {
	u := &Updater{GOOS: "darwin", GOARCH: "arm64"}
	if got := u.ArchiveName("1.4.0"); got != "rail0_1.4.0_darwin_arm64.tar.gz" {
		t.Errorf("got %q", got)
	}
	u = &Updater{GOOS: "windows", GOARCH: "amd64"}
	if got := u.ArchiveName("1.4.0"); got != "rail0_1.4.0_windows_amd64.zip" {
		t.Errorf("got %q", got)
	}
}

// releaseServer serves a GitHub-shaped latest release with one archive and its
// checksums.txt. Tweak lets a test corrupt either before serving.
type releaseServer struct {
	*httptest.Server
	archiveName string
	archive     []byte
	sums        string
}

func newReleaseServer(t *testing.T, u *Updater, version string, archive []byte) *releaseServer {
	t.Helper()
	rs := &releaseServer{archiveName: u.ArchiveName(version), archive: archive}
	sum := sha256.Sum256(archive)
	rs.sums = fmt.Sprintf("%s  %s\n%s  rail0_%s_other_arch.tar.gz\n", hex.EncodeToString(sum[:]), rs.archiveName, strings.Repeat("0", 64), version)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/commercelayer/rail0-cli/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		base := rs.URL
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v" + version,
			"assets": []map[string]any{
				{"name": rs.archiveName, "browser_download_url": base + "/dl/archive", "size": len(rs.archive)},
				{"name": "checksums.txt", "browser_download_url": base + "/dl/sums", "size": len(rs.sums)},
			},
		})
	})
	mux.HandleFunc("/dl/archive", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(rs.archive)))
		_, _ = w.Write(rs.archive)
	})
	mux.HandleFunc("/dl/sums", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(rs.sums)) })
	rs.Server = httptest.NewServer(mux)
	t.Cleanup(rs.Close)
	u.APIBase = rs.URL
	return rs
}

func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write(body)
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(body)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func installedBinary(t *testing.T, content string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "rail0")
	if err := os.WriteFile(exe, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func newTestUpdater() *Updater {
	u := New(&http.Client{Timeout: 5 * time.Second})
	u.GOOS, u.GOARCH = "linux", "amd64"
	return u
}

func TestInstallReplacesTheBinaryAndReportsProgress(t *testing.T) {
	u := newTestUpdater()
	archive := tarGz(t, map[string][]byte{
		"README.md":          []byte("readme"),
		"completions/_rail0": []byte("#compdef"),
		"rail0":              []byte("NEW BINARY"),
		"man/man1/rail0.1":   []byte("man"),
	})
	newReleaseServer(t, u, "1.4.0", archive)
	exe := installedBinary(t, "OLD BINARY")

	rel, err := u.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "1.4.0" {
		t.Fatalf("version %q, want 1.4.0 (tag without the v)", rel.Version)
	}
	var lastDone, lastTotal int64
	if err := u.Install(context.Background(), rel, exe, func(d, tot int64) { lastDone, lastTotal = d, tot }); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(exe)
	if string(got) != "NEW BINARY" {
		t.Errorf("binary is %q, want the one from the archive", got)
	}
	if info, _ := os.Stat(exe); info.Mode().Perm() != 0o755 {
		t.Errorf("mode %v, want the old binary's 0755 kept", info.Mode().Perm())
	}
	if lastTotal != int64(len(archive)) || lastDone != lastTotal {
		t.Errorf("progress ended at %d/%d, want %d/%d", lastDone, lastTotal, len(archive), len(archive))
	}
}

// Verification happens before anything on disk changes: a tampered or truncated
// download must leave the installed binary exactly as it was.
func TestInstallRefusesAChecksumMismatch(t *testing.T) {
	u := newTestUpdater()
	rs := newReleaseServer(t, u, "1.4.0", tarGz(t, map[string][]byte{"rail0": []byte("NEW")}))
	rs.archive = tarGz(t, map[string][]byte{"rail0": []byte("TAMPERED")})
	exe := installedBinary(t, "OLD BINARY")

	rel, _ := u.Latest(context.Background())
	err := u.Install(context.Background(), rel, exe, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v, want a checksum mismatch", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "OLD BINARY" {
		t.Errorf("binary changed to %q despite the mismatch", got)
	}
}

func TestInstallRefusesAnArchiveWithoutAChecksum(t *testing.T) {
	u := newTestUpdater()
	rs := newReleaseServer(t, u, "1.4.0", tarGz(t, map[string][]byte{"rail0": []byte("NEW")}))
	rs.sums = strings.Repeat("0", 64) + "  something_else.tar.gz\n"
	exe := installedBinary(t, "OLD")

	rel, _ := u.Latest(context.Background())
	if err := u.Install(context.Background(), rel, exe, nil); err == nil || !strings.Contains(err.Error(), "no checksum") {
		t.Fatalf("err = %v, want a refusal to install unverified", err)
	}
}

func TestInstallFailsWithoutABuildForThisPlatform(t *testing.T) {
	u := newTestUpdater()
	newReleaseServer(t, u, "1.4.0", tarGz(t, map[string][]byte{"rail0": []byte("NEW")}))
	u.GOARCH = "riscv64" // asked for after the server listed amd64 only

	rel, _ := u.Latest(context.Background())
	if err := u.Install(context.Background(), rel, installedBinary(t, "OLD"), nil); err == nil || !strings.Contains(err.Error(), "no build for linux/riscv64") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallExtractsTheWindowsZip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Replace takes the rename-away path on Windows; covered by the zip read alone")
	}
	u := newTestUpdater()
	u.GOOS = "windows"
	newReleaseServer(t, u, "1.4.0", zipOf(t, map[string][]byte{"README.md": []byte("r"), "rail0.exe": []byte("NEW EXE")}))
	exe := installedBinary(t, "OLD")

	rel, _ := u.Latest(context.Background())
	if err := u.Install(context.Background(), rel, exe, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "NEW EXE" {
		t.Errorf("got %q", got)
	}
}

// Until the repository is public GitHub answers an anonymous caller with 404; the
// error must say so rather than look like a network fault.
func TestLatestExplainsAMissingRelease(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	u := newTestUpdater()
	u.APIBase = srv.URL

	if _, err := u.Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "no published rail0 release") {
		t.Fatalf("err = %v", err)
	}
}

// The r0 alias is a symlink to rail0: replacing the RESOLVED file keeps the link
// pointing at the new binary instead of turning it into a stale copy.
func TestReplaceKeepsASymlinkPointingAtTheNewBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	exe := installedBinary(t, "OLD")
	alias := filepath.Join(filepath.Dir(exe), "r0")
	if err := os.Symlink(exe, alias); err != nil {
		t.Fatal(err)
	}
	if err := Replace(exe, []byte("NEW")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(alias); string(got) != "NEW" {
		t.Errorf("alias reads %q, want the new binary", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(exe)); len(entries) != 2 {
		t.Errorf("left %d entries behind, want just rail0 and r0", len(entries))
	}
}

func TestReplaceReportsAnUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	exe := installedBinary(t, "OLD")
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := Replace(exe, []byte("NEW"))
	if err == nil || !strings.Contains(err.Error(), ErrNotWritable.Error()) {
		t.Fatalf("err = %v, want ErrNotWritable", err)
	}
}

func TestCheckDueOncePerInterval(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("LocalAppData", filepath.Join(home, "AppData"))
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	if !CheckDue(now) {
		t.Fatal("never checked: want due")
	}
	MarkChecked(now)
	if CheckDue(now.Add(CheckInterval - time.Minute)) {
		t.Error("checked less than a day ago: want not due")
	}
	if !CheckDue(now.Add(CheckInterval)) {
		t.Error("a day later: want due")
	}
	if !CheckDue(now.Add(-time.Hour)) {
		t.Error("clock set back before the last check: want due, not silenced")
	}
}
