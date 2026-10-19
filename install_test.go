// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeCurl stands in for curl so install.sh can be exercised offline.
// It models github.com, not the API: FAKE_LATEST is the stable tag that
// /releases/latest redirects to (empty = only prereleases exist, so GitHub
// redirects to /releases); FAKE_LIST is the newest tag in releases.atom
// (empty = feed unavailable, curl -f → 22). FAKE_OFFLINE=1 fails every call.
// Downloads copy FAKE_TARBALL to the -o path. Every URL is appended to FAKE_LOG.
const fakeCurl = `#!/bin/sh
url=""; out=""; write=""
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
		if env == "NO_GH=1" {
			noGH = true
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
		if env == "NO_GH=1" {
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

	cmd := exec.Command("sh", append([]string{"install.sh"}, args...)...)
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
		"mkdir", "dirname", "cat", "sha256sum", "shasum", "python3", "cp", "date"} {
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
	// Explicit version: no lookup, only the archive and its checksums.txt.
	if len(r.urls) != 2 {
		t.Errorf("explicit version should only download the archive and checksums, got curl calls: %v", r.urls)
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

func TestInstallScriptShellcheck(t *testing.T) {
	path, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Skip("shellcheck not installed")
	}
	if out, err := exec.Command(path, "install.sh").CombinedOutput(); err != nil {
		t.Fatalf("shellcheck install.sh: %v\n%s", err, out)
	}
}

func TestInstallExplicitUpshiftPersistsFlag(t *testing.T) {
	r := runInstall(t, "v1.0.0", "", "--explicit-upshift")
	assertInstalled(t, r)
	body, err := os.ReadFile(filepath.Join(r.home, ".downshift", "session-models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("session file %s: %v", body, err)
	}
	if doc["explicit_upshift"] != true {
		t.Fatalf("explicit_upshift = %#v, want true", doc["explicit_upshift"])
	}
}

func TestInstallExplicitUpshiftMergesExistingFile(t *testing.T) {
	r := runInstallEnv(t, "", "", []string{`SEED_SESSION={"codex":["gpt-6-luna","gpt-6-sol"]}`}, "v0.1.0-beta.1", "--explicit-upshift")
	assertInstalled(t, r)
	body, err := os.ReadFile(filepath.Join(r.home, ".downshift", "session-models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ExplicitUpshift bool     `json:"explicit_upshift"`
		Codex           []string `json:"codex"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("session file %s: %v", body, err)
	}
	if !doc.ExplicitUpshift {
		t.Fatalf("explicit_upshift missing in %s", body)
	}
	if len(doc.Codex) != 2 || doc.Codex[0] != "gpt-6-luna" || doc.Codex[1] != "gpt-6-sol" {
		t.Fatalf("codex list = %#v", doc.Codex)
	}
}

// Without a session allowlist the hook never rewrites, so the installer must
// tell the user to create one instead of finishing a silent no-op install.
func TestInstallExplainsSessionAllowlist(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	if !strings.Contains(r.output, "session-models.json") {
		t.Fatalf("installer must explain session-models.json, output:\n%s", r.output)
	}
}

func sha256Of(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// The installer downloads checksums.txt and verifies the archive before it
// installs anything. A wrong digest or a missing checksums file is a hard
// stop unless the user opts out.
func TestInstallVerifiesChecksum(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	found := false
	for _, u := range r.urls {
		if strings.HasSuffix(u, "/releases/download/v1.0.0/checksums.txt") {
			found = true
		}
	}
	if !found || !strings.Contains(r.output, "Checksum verified") {
		t.Fatalf("checksums.txt not fetched or not verified; urls=%v\noutput:\n%s", r.urls, r.output)
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"FAKE_BAD_SHA=" + strings.Repeat("0", 64)})
	if r.code != 1 || !strings.Contains(r.output, "checksum mismatch") {
		t.Fatalf("exit %d, want 1 with a mismatch message\noutput:\n%s", r.code, r.output)
	}
	if _, err := os.Stat(filepath.Join(r.binDir, "downshift")); err == nil {
		t.Fatal("binary must not be installed on a checksum mismatch")
	}
}

func TestInstallMissingChecksumsNeedsOptOut(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"FAKE_NO_CHECKSUMS=1"})
	if r.code != 1 || !strings.Contains(r.output, "DOWNSHIFT_SKIP_CHECKSUM=1") {
		t.Fatalf("exit %d, want 1 naming the opt-out\noutput:\n%s", r.code, r.output)
	}
	r = runInstallEnv(t, "v1.0.0", "", []string{"FAKE_NO_CHECKSUMS=1", "DOWNSHIFT_SKIP_CHECKSUM=1"})
	assertInstalled(t, r)
	if !strings.Contains(r.output, "checksum verification skipped") {
		t.Fatalf("opt-out must be loud\noutput:\n%s", r.output)
	}
}

// Build provenance is checked when gh exists and never blocks the install
// unless the operator asks for it.
func TestInstallVerifiesAttestationWithGH(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"FAKE_GH=ok"})
	assertInstalled(t, r)
	if !strings.Contains(r.output, "Build provenance verified") {
		t.Fatalf("attestation not reported\noutput:\n%s", r.output)
	}
	log, _ := os.ReadFile(r.ghLog)
	if !strings.Contains(string(log), "attestation verify") || !strings.Contains(string(log), "--repo tiagovilasboas/downshift") {
		t.Fatalf("gh called as %q", log)
	}
}

func TestInstallWarnsButInstallsWhenAttestationFails(t *testing.T) {
	r := runInstall(t, "v1.0.0", "")
	assertInstalled(t, r)
	if !strings.Contains(r.output, "could not be verified") || !strings.Contains(r.output, "sha256 check only") {
		t.Fatalf("expected a warning\noutput:\n%s", r.output)
	}
}

func TestInstallWarnsButInstallsWithoutGH(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"NO_GH=1"})
	assertInstalled(t, r)
	if !strings.Contains(r.output, "gh CLI is not installed") || !strings.Contains(r.output, "Checksum verified") {
		t.Fatalf("expected a warning and the sha256 pass\noutput:\n%s", r.output)
	}
}

func TestInstallRequireAttestationBlocks(t *testing.T) {
	for name, env := range map[string][]string{
		"gh fails":   {"DOWNSHIFT_REQUIRE_ATTESTATION=1"},
		"gh missing": {"DOWNSHIFT_REQUIRE_ATTESTATION=1", "NO_GH=1"},
	} {
		r := runInstallEnv(t, "v1.0.0", "", env)
		if r.code != 1 || !strings.Contains(r.output, "DOWNSHIFT_REQUIRE_ATTESTATION=1") {
			t.Errorf("%s: exit %d, want 1\noutput:\n%s", name, r.code, r.output)
		}
		if _, err := os.Stat(filepath.Join(r.binDir, "downshift")); err == nil {
			t.Errorf("%s: binary installed despite the requirement", name)
		}
	}
	ok := runInstallEnv(t, "v1.0.0", "", []string{"DOWNSHIFT_REQUIRE_ATTESTATION=1", "FAKE_GH=ok"})
	assertInstalled(t, ok)
}

func TestInstallSkipAttestationDoesNotCallGH(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"DOWNSHIFT_SKIP_ATTESTATION=1"})
	assertInstalled(t, r)
	if _, err := os.Stat(r.ghLog); err == nil {
		t.Fatal("gh was called although the check was skipped")
	}
}
