// Package selfupdate replaces the running rail0 binary with the latest GitHub release.
//
// The releases are the ones GoReleaser publishes (.goreleaser.yaml): one archive per
// platform named rail0_<version>_<os>_<arch>.tar.gz (.zip on Windows), plus a
// checksums.txt with the SHA-256 of every archive. An update is:
//
//  1. read the latest release from the GitHub API (prereleases are never "latest");
//  2. download this platform's archive, reporting progress;
//  3. verify its SHA-256 against checksums.txt — a mismatch aborts before anything on
//     disk changes;
//  4. extract the rail0 binary and swap it in place of the running one (see Replace).
//
// Everything here is standard library: no third-party updater, so what runs on the
// user's machine is exactly what is in this file.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// DefaultAPIBase and DefaultRepo locate the releases. Fields on Updater so tests can
// point them at a local server.
const (
	DefaultAPIBase = "https://api.github.com"
	DefaultRepo    = "commercelayer/rail0-cli"
)

// checksumsName is the file GoReleaser writes next to the archives (checksum.name_template).
const checksumsName = "checksums.txt"

// maxBinarySize bounds what extraction will write, so a corrupted or hostile archive
// cannot fill the disk. The real binary is ~40MB; the checksum has already been
// verified by then, so this is belt and braces.
const maxBinarySize = 512 << 20

// Release is the part of a GitHub release an update needs.
type Release struct {
	// Version is the tag without its leading "v" — the form GoReleaser uses in asset
	// names and stamps into the binary.
	Version string
	Assets  map[string]Asset
}

// Asset is one downloadable file of a release.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// Progress is called while an archive downloads: done bytes so far, and total bytes
// (0 when the server did not say).
type Progress func(done, total int64)

// Updater finds, downloads and installs releases.
type Updater struct {
	Client  *http.Client
	APIBase string
	Repo    string
	GOOS    string
	GOARCH  string
}

// New returns an Updater for the published CLI releases on this platform.
func New(client *http.Client) *Updater {
	return &Updater{
		Client:  client,
		APIBase: DefaultAPIBase,
		Repo:    DefaultRepo,
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
	}
}

// Latest reads the most recent non-prerelease release.
func (u *Updater) Latest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(u.APIBase, "/"), u.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reading the latest release: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		// No release published yet — or the repository is not public, which GitHub
		// also answers with 404 to an anonymous caller.
		return nil, errors.New("no published rail0 release was found (the release repository may not be public yet)")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("reading the latest release: GitHub answered HTTP %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("reading the latest release: %w", err)
	}
	rel := &Release{Version: strings.TrimPrefix(body.TagName, "v"), Assets: map[string]Asset{}}
	for _, a := range body.Assets {
		rel.Assets[a.Name] = Asset{Name: a.Name, URL: a.URL, Size: a.Size}
	}
	return rel, nil
}

// ArchiveName is this platform's archive in a release, per the GoReleaser
// name_template: rail0_<version>_<os>_<arch>.tar.gz, .zip on Windows.
func (u *Updater) ArchiveName(version string) string {
	ext := ".tar.gz"
	if u.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("rail0_%s_%s_%s%s", version, u.GOOS, u.GOARCH, ext)
}

// Install downloads rel's archive for this platform, verifies it, and replaces the
// binary at exePath with the one inside.
//
// The order is what makes it safe to interrupt: the archive goes to a temp file and is
// verified in full before exePath is touched, and the swap itself is a rename (see
// Replace). A failure at any step leaves the installed binary as it was.
func (u *Updater) Install(ctx context.Context, rel *Release, exePath string, progress Progress) error {
	archive, ok := rel.Assets[u.ArchiveName(rel.Version)]
	if !ok {
		return fmt.Errorf("release %s has no build for %s/%s", rel.Version, u.GOOS, u.GOARCH)
	}
	sums, ok := rel.Assets[checksumsName]
	if !ok {
		return fmt.Errorf("release %s has no %s; refusing to install an unverified binary", rel.Version, checksumsName)
	}

	var sumsBuf bytes.Buffer
	if err := u.download(ctx, sums.URL, &sumsBuf, nil); err != nil {
		return fmt.Errorf("downloading %s: %w", checksumsName, err)
	}
	want, err := checksumFor(sumsBuf.Bytes(), archive.Name)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "rail0-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hash := sha256.New()
	if err := u.download(ctx, archive.URL, io.MultiWriter(tmp, hash), progress); err != nil {
		return fmt.Errorf("downloading %s: %w", archive.Name, err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s (expected %s, got %s); nothing was installed", archive.Name, want, got)
	}

	bin, err := extractBinary(tmp, strings.HasSuffix(archive.Name, ".zip"))
	if err != nil {
		return err
	}
	return Replace(exePath, bin)
}

// download streams url into w, calling progress as bytes arrive.
func (u *Updater) download(ctx context.Context, url string, w io.Writer, progress Progress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := u.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var r io.Reader = resp.Body
	if progress != nil {
		r = &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
		progress(0, max(resp.ContentLength, 0))
	}
	_, err = io.Copy(w, r)
	return err
}

type progressReader struct {
	r     io.Reader
	done  int64
	total int64
	fn    Progress
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	p.fn(p.done, max(p.total, 0))
	return n, err
}

// checksumFor finds name's SHA-256 in a checksums.txt ("<hex>  <file>" per line).
func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s lists no checksum for %s; refusing to install an unverified binary", checksumsName, name)
}

// extractBinary returns the rail0 executable inside an archive. Only an entry whose
// BASE name is the binary counts, wherever it sits: GoReleaser puts it at the root,
// next to README, completions/ and man/.
func extractBinary(f *os.File, isZip bool) ([]byte, error) {
	name := "rail0"
	if isZip {
		name = "rail0.exe"
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if isZip {
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		zr, err := zip.NewReader(f, info.Size())
		if err != nil {
			return nil, fmt.Errorf("reading the archive: %w", err)
		}
		for _, zf := range zr.File {
			if filepath.Base(zf.Name) == name && !zf.FileInfo().IsDir() {
				rc, err := zf.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return readBounded(rc)
			}
		}
		return nil, fmt.Errorf("the archive contains no %s", name)
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("reading the archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("the archive contains no %s", name)
		}
		if err != nil {
			return nil, fmt.Errorf("reading the archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == name {
			return readBounded(tr)
		}
	}
}

func readBounded(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBinarySize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBinarySize {
		return nil, errors.New("the binary in the archive is implausibly large; nothing was installed")
	}
	return b, nil
}

// Replace swaps the executable at exePath for bin.
//
// The new binary is written to a temp file in the SAME directory and renamed over the
// old one: a rename within one filesystem is atomic, so the path always holds either the
// complete old binary or the complete new one — never a half-written file, even if the
// process is killed mid-way. The running process keeps its already-mapped image, so the
// command that triggered the update finishes on the old code.
//
// Windows will not rename over a running executable, but it does allow renaming the
// running one away: it is moved to <exe>.old first (removed by CleanupOld on a later
// run). A failure to put the new one in place moves the old one back.
//
// exePath should be the resolved path (see Executable) so a symlink such as the r0
// alias keeps pointing at the updated file instead of being replaced by a copy.
func Replace(exePath string, bin []byte) error {
	dir := filepath.Dir(exePath)
	mode := os.FileMode(0o755)
	if info, err := os.Stat(exePath); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".rail0-new-*")
	if err != nil {
		return notWritable(dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		old := exePath + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exePath, old); err != nil {
			return notWritable(dir, err)
		}
		if err := os.Rename(tmpName, exePath); err != nil {
			_ = os.Rename(old, exePath)
			return err
		}
		return nil
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		return notWritable(dir, err)
	}
	return nil
}

// ErrNotWritable is returned when the binary's directory cannot be written — the
// usual case for a binary installed into /usr/local/bin by `make install`.
var ErrNotWritable = errors.New("the rail0 install directory is not writable")

func notWritable(dir string, err error) error {
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("%w (%s): %v", ErrNotWritable, dir, err)
	}
	return err
}

// ManagedBy names the package manager that owns the binary at exePath, or "" when none
// does. Only Homebrew for now: its casks live under …/Caskroom/ and formulas under
// …/Cellar/, on macOS and Linux alike.
//
// Such a binary must not be replaced in place. The package manager records the version
// it installed; swapping the file behind its back leaves `brew list` and `brew upgrade`
// believing the old version is there, and the next `brew upgrade` would silently put it
// back. The upgrade has to go through the package manager instead.
func ManagedBy(exePath string) string {
	p := filepath.ToSlash(exePath)
	if strings.Contains(p, "/Caskroom/") || strings.Contains(p, "/Cellar/") {
		return "Homebrew"
	}
	return ""
}

// UpgradeCommand is the command that upgrades rail0 through its package manager.
func UpgradeCommand(manager string) string {
	if manager == "Homebrew" {
		return "brew upgrade rail0"
	}
	return "rail0 update"
}

// Executable is the path of the running binary with symlinks resolved — the file an
// update must replace.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// CleanupOld removes the <exe>.old a Windows update leaves behind (it could not be
// deleted while it was the running process). Best-effort, and a no-op elsewhere.
func CleanupOld() {
	if runtime.GOOS != "windows" {
		return
	}
	if exe, err := Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}
}

// Updatable reports whether current is a release version that can be compared with
// the published releases — false for local builds (see Newer), which the automatic
// check skips without a network call.
func Updatable(current string) bool {
	_, ok := parseVersion(current)
	return ok
}

// Newer reports whether latest is a higher version than current. Both are
// "MAJOR.MINOR.PATCH" with an optional leading "v" and optional "-prerelease". A local
// build is never considered older, so it is never offered an update over itself: "dev",
// anything unparsable, and the `git describe` form the Makefile stamps
// (v1.3.0-5-gabc1234[-dirty], i.e. commits AFTER a tag, which would otherwise read as a
// prerelease of 1.3.0 and be "updated" back to it).
func Newer(current, latest string) bool {
	c, ok1 := parseVersion(current)
	l, ok2 := parseVersion(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range 3 {
		if l.nums[i] != c.nums[i] {
			return l.nums[i] > c.nums[i]
		}
	}
	// Same numbers: a release beats its own prerelease (1.4.0 > 1.4.0-rc.1). Two
	// prereleases are compared as strings, which is enough for rc.N ordering here.
	switch {
	case c.pre != "" && l.pre == "":
		return true
	case c.pre != "" && l.pre != "":
		return l.pre > c.pre
	default:
		return false
	}
}

type version struct {
	nums [3]int
	pre  string
}

func parseVersion(s string) (version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	core, pre, _ := strings.Cut(s, "-")
	core, _, _ = strings.Cut(core, "+") // build metadata never orders
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var v version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, false
		}
		v.nums[i] = n
	}
	if gitDescribe.MatchString(pre) {
		return version{}, false
	}
	v.pre = pre
	return v, true
}

// gitDescribe matches the suffix `git describe --tags --dirty` appends after a tag:
// "<commits>-g<hash>", optionally "-dirty" — or a bare "dirty" on a tagged, modified tree.
var gitDescribe = regexp.MustCompile(`^(\d+-g[0-9a-f]+(-dirty)?|dirty)$`)
