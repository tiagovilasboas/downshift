// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"archive/tar"
	"compress/gzip"
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
  */releases/download/*)
    cp "$FAKE_TARBALL" "$out" ;;
  *) exit 22 ;;
esac
`

type installResult struct {
	code   int
	output string
	urls   []string
	binDir string
	home   string
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
	tarball := filepath.Join(tmp, "release.tar.gz")
	writeTarball(t, tarball)
	logFile := filepath.Join(tmp, "curl.log")
	binDir := filepath.Join(tmp, "bin")

	var seed string
	keptEnv := make([]string, 0, len(extraEnv))
	for _, env := range extraEnv {
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
		"PATH=" + fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + tmp,
		"DOWNSHIFT_INSTALL_DIR=" + binDir,
		"FAKE_LATEST=" + latest,
		"FAKE_LIST=" + list,
		"FAKE_TARBALL=" + tarball,
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
	}
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
	if len(r.urls) != 1 {
		t.Errorf("explicit version should only download, got curl calls: %v", r.urls)
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
