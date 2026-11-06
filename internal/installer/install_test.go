// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installScript is the repository-root installer, two levels up from this package.
var installScript = filepath.Join("..", "..", "install.sh")

// fakeCurl stands in for curl so install.sh can be exercised offline.
// It models github.com, not the API: FAKE_LATEST is the stable tag that
// /releases/latest redirects to (empty = only prereleases exist, so GitHub
// redirects to /releases); FAKE_LIST is the newest tag in releases.atom
// (empty = feed unavailable, curl -f → 22). FAKE_OFFLINE=1 fails every call.
// Downloads copy FAKE_TARBALL to the -o path. Every URL is appended to FAKE_LOG.
const fakeCurl = `#!/bin/sh
url=""; out=""; write=""
echo "$*" >> "$FAKE_LOG.args"
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift ;;
    -w) write="$2"; shift ;;
    http*) url="$1" ;;
  esac
  shift
done
echo "$url" >> "$FAKE_LOG"
[ "$FAKE_OFFLINE" = 1 ] && exit 6
case "$url" in
  https://api.github.com/*)
    echo "API must not be called" >&2; exit 22 ;;
  */releases/latest)
    base="${url%/latest}"
    if [ -n "$FAKE_LATEST" ]; then final="$base/tag/$FAKE_LATEST"; else final="$base"; fi
    [ "$write" = '%{url_effective}' ] && printf '%s' "$final" ;;
  */releases.atom)
    [ -n "$FAKE_LIST" ] || exit 22
    base="${url%.atom}"
    printf '<feed>\n  <link rel="alternate" href="%s"/>\n' "$base"
    printf '  <entry>\n    <link rel="alternate" type="text/html" href="%s/tag/%s"/>\n  </entry>\n' "$base" "$FAKE_LIST"
    printf '  <entry>\n    <link rel="alternate" type="text/html" href="%s/tag/v0.0.1-alpha.1"/>\n  </entry>\n</feed>\n' "$base" ;;
  */releases/download/*/checksums.txt)
    [ "$FAKE_NO_CHECKSUMS" = 1 ] && exit 22
    ver="${url#*/releases/download/}"; ver="${ver%%/*}"; ver="${ver#v}"
    sha="$FAKE_SHA"; [ -n "$FAKE_BAD_SHA" ] && sha="$FAKE_BAD_SHA"
    : > "$out"
    for os in linux darwin; do for arch in amd64 arm64; do
      printf '%s  downshift_%s_%s_%s.tar.gz\n' "$sha" "$ver" "$os" "$arch" >> "$out"
    done; done ;;
  */releases/download/*)
    [ "$FAKE_NO_ARCHIVE" = 1 ] && exit 22
    cp "$FAKE_TARBALL" "$out" ;;
  *) exit 22 ;;
esac
`

// fakeGH stands in for the gh CLI: FAKE_GH=ok verifies, anything else fails.
// Every call is appended to FAKE_GH_LOG.
const fakeGH = `#!/bin/sh
echo "$*" >> "$FAKE_GH_LOG"
[ "$FAKE_GH" = ok ] && exit 0
echo "no attestation found" >&2
exit 1
`

type installResult struct {
	code   int
	output string
	urls   []string
	binDir string
	home   string
	ghLog  string
}

func runInstall(t *testing.T, latest, list string, args ...string) installResult {
	t.Helper()
	return runInstallEnv(t, latest, list, nil, args...)
}

func runInstallEnv(t *testing.T, latest, list string, extraEnv []string, args ...string) installResult {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh targets macOS/Linux")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "fakebin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte(fakeCurl), 0o755); err != nil {
		t.Fatal(err)
	}
	// A fake gh always shadows a real one, so no test reaches the network.
	// NO_GH=1 builds a PATH that has the tools install.sh needs and no gh.
	noGH := false
	for _, env := range extraEnv {
		switch env {
		case "NO_GH=1":
			noGH = true
		case "STALE_DOWNSHIFT=1":
			// An older downshift first on PATH, as ~/go/bin often is.
			if err := os.WriteFile(filepath.Join(fakeBin, "downshift"), []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := fakeBin + string(os.PathListSeparator) + os.Getenv("PATH")
	if noGH {
		path = toolOnlyPath(t, fakeBin)
	} else if err := os.WriteFile(filepath.Join(fakeBin, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}
	ghLog := filepath.Join(tmp, "gh.log")
	tarball := filepath.Join(tmp, "release.tar.gz")
	writeTarball(t, tarball)
	logFile := filepath.Join(tmp, "curl.log")
	binDir := filepath.Join(tmp, "bin")

	var seed string
	keptEnv := make([]string, 0, len(extraEnv))
	for _, env := range extraEnv {
		if env == "NO_GH=1" || env == "STALE_DOWNSHIFT=1" {
			continue
		}
		if strings.HasPrefix(env, "SEED_SESSION=") {
			seed = strings.TrimPrefix(env, "SEED_SESSION=")
			continue
		}
		keptEnv = append(keptEnv, env)
	}
	if seed != "" {
		sessionDir := filepath.Join(tmp, ".downshift")
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sessionDir, "session-models.json"), []byte(seed), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("sh", append([]string{installScript}, args...)...)
	cmd.Env = []string{
		"PATH=" + path,
		"FAKE_GH_LOG=" + ghLog,
		"HOME=" + tmp,
		"DOWNSHIFT_INSTALL_DIR=" + binDir,
		"FAKE_LATEST=" + latest,
		"FAKE_LIST=" + list,
		"FAKE_TARBALL=" + tarball,
		"FAKE_SHA=" + sha256Of(t, tarball),
		"FAKE_LOG=" + logFile,
	}
	cmd.Env = append(cmd.Env, keptEnv...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running install.sh: %v", err)
		}
		code = exitErr.ExitCode()
	}
	logged, _ := os.ReadFile(logFile)
	return installResult{
		code:   code,
		output: string(out),
		urls:   strings.Fields(string(logged)),
		binDir: binDir,
		home:   tmp,
		ghLog:  ghLog,
	}
}

// toolOnlyPath returns a PATH holding fakeBin plus symlinks to the external
// tools install.sh uses, and no gh.
func toolOnlyPath(t *testing.T, fakeBin string) string {
	t.Helper()
	dir := filepath.Join(filepath.Dir(fakeBin), "tools")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"sh", "uname", "tr", "mktemp", "rm", "grep", "awk", "head", "sed", "tar", "mv", "chmod",
		"mkdir", "dirname", "cat", "sha256sum", "shasum", "python3", "cp", "date", "gzip", "gunzip", "env"} {
		if p, err := exec.LookPath(tool); err == nil {
			if err := os.Symlink(p, filepath.Join(dir, tool)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return fakeBin + string(os.PathListSeparator) + dir
}

func writeTarball(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho fake-downshift\n")
	if err := tw.WriteHeader(&tar.Header{Name: "downshift", Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func downloadURL(t *testing.T, r installResult) string {
	t.Helper()
	for _, u := range r.urls {
		if strings.Contains(u, "/releases/download/") {
			return u
		}
	}
	t.Fatalf("no download attempted; curl calls: %v\noutput:\n%s", r.urls, r.output)
	return ""
}

func assertInstalled(t *testing.T, r installResult) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("install.sh exit %d, want 0\noutput:\n%s", r.code, r.output)
	}
	if _, err := os.Stat(filepath.Join(r.binDir, "downshift")); err != nil {
		t.Fatalf("binary not installed in %s: %v", r.binDir, err)
	}
}

func assertNoAPICalls(t *testing.T, r installResult) {
	t.Helper()
	for _, u := range r.urls {
		if strings.Contains(u, "api.github.com") {
			t.Errorf("install.sh called the rate-limited GitHub API: %v", r.urls)
		}
	}
}

// Regression: while only prereleases exist, /releases/latest does not point at
// a tag and the one-line installer exited 1 for every user.
func TestInstallFallsBackToPrereleaseWhenNoStableRelease(t *testing.T) {
	r := runInstall(t, "", "v0.1.0-beta.1")
	assertInstalled(t, r)
	assertNoAPICalls(t, r)
	u := downloadURL(t, r)
	if !strings.Contains(u, "/releases/download/v0.1.0-beta.1/downshift_0.1.0-beta.1_") {
		t.Errorf("download URL = %s, want the v0.1.0-beta.1 archive", u)
	}
}

func TestInstallPrefersLatestStableRelease(t *testing.T) {
	r := runInstall(t, "v1.0.0", "v1.1.0-beta.1")
	assertInstalled(t, r)
	assertNoAPICalls(t, r)
	if u := downloadURL(t, r); !strings.Contains(u, "/releases/download/v1.0.0/") {
		t.Errorf("download URL = %s, want v1.0.0", u)
	}
	for _, u := range r.urls {
		if strings.Contains(u, "releases.atom") {
			t.Errorf("release feed read although /releases/latest answered: %v", r.urls)
		}
	}
}

// Regression: the installer resolved "latest" through api.github.com, whose
// anonymous rate limit (60 requests/hour per IP) made it fail on shared IPs.
func TestInstallResolvesLatestWithoutGitHubAPI(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	assertNoAPICalls(t, r)
	if len(r.urls) == 0 || r.urls[0] != "https://github.com/tiagovilasboas/downshift/releases/latest" {
		t.Errorf("first lookup = %v, want the github.com /releases/latest redirect", r.urls)
	}
}

func TestInstallExplicitVersionSkipsLookup(t *testing.T) {
	r := runInstall(t, "", "", "v0.1.0-beta.1")
	assertInstalled(t, r)
	// Explicit version: no lookup, only the archive (HEAD probe, then download) and its checksums.txt.
	if len(r.urls) != 3 {
		t.Errorf("explicit version should only probe and download the archive and checksums, got curl calls: %v", r.urls)
	}
	for _, u := range r.urls {
		if strings.Contains(u, "/releases/latest") || strings.Contains(u, "releases.atom") {
			t.Errorf("explicit version must not look up releases: %v", r.urls)
		}
	}
	if u := downloadURL(t, r); !strings.Contains(u, "/releases/download/v0.1.0-beta.1/downshift_0.1.0-beta.1_") {
		t.Errorf("download URL = %s, want the v0.1.0-beta.1 archive", u)
	}
}

func TestInstallFailsClearlyWhenNoReleaseFound(t *testing.T) {
	r := runInstall(t, "", "")
	if r.code != 1 {
		t.Fatalf("install.sh exit %d, want 1\noutput:\n%s", r.code, r.output)
	}
	if !strings.Contains(r.output, "could not determine latest version") {
		t.Errorf("missing actionable error, got:\n%s", r.output)
	}
}

func TestInstallFailsClearlyWhenOffline(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "v1.0.0", []string{"FAKE_OFFLINE=1"})
	if r.code != 1 {
		t.Fatalf("install.sh exit %d, want 1\noutput:\n%s", r.code, r.output)
	}
	if !strings.Contains(r.output, "could not determine latest version") {
		t.Errorf("missing actionable error, got:\n%s", r.output)
	}
}
