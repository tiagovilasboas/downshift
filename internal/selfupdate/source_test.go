// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func releaseServer(t *testing.T, latest string, assets map[string][]byte) Source {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if latest == "" {
			http.Redirect(w, r, "/releases", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/releases/tag/"+latest, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := assets[filepath.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return Source{Base: srv.URL, Client: srv.Client()}
}

func TestSourceLatestAndDownload(t *testing.T) {
	src := releaseServer(t, "v0.15.0", map[string][]byte{"checksums.txt": []byte("x")})
	tag, err := src.LatestTag()
	if err != nil || tag != "v0.15.0" {
		t.Fatalf("LatestTag = %q, %v", tag, err)
	}
	body, err := src.Download(tag, "checksums.txt")
	if err != nil || string(body) != "x" {
		t.Fatalf("Download = %q, %v", body, err)
	}
	if _, err := src.Download(tag, "missing.zip"); err == nil {
		t.Error("a 404 must fail")
	}
}

func TestSourceRefusesNoReleaseAndPlainHTTP(t *testing.T) {
	if _, err := releaseServer(t, "", nil).LatestTag(); err == nil {
		t.Error("no latest release must fail")
	}
	plain := Source{Base: "http://example.invalid"}
	if _, err := plain.LatestTag(); err == nil {
		t.Error("a plain http base must be refused")
	}
	if _, err := plain.Download("v1.0.0", "a.zip"); err == nil {
		t.Error("a plain http base must be refused")
	}
}

func TestReplaceKeepsModeAndFollowsLinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "downshift")
	if err := os.WriteFile(real, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := real
	if runtime.GOOS != "windows" {
		target = filepath.Join(dir, "link")
		if err := os.Symlink(real, target); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Replace(target, []byte("new"))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(real); got != want {
		t.Errorf("replaced %q, want the resolved file %q", got, want)
	}
	if b, _ := os.ReadFile(real); string(b) != "new" {
		t.Errorf("content = %q", b)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(real); fi.Mode().Perm() != 0o755 {
			t.Errorf("mode = %v", fi.Mode().Perm())
		}
		if fi, _ := os.Lstat(target); fi.Mode()&os.ModeSymlink == 0 {
			t.Error("the link must stay a link")
		}
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".downshift-update-*"))
	if len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}

func TestReplaceMissingTargetFails(t *testing.T) {
	if _, err := Replace(filepath.Join(t.TempDir(), "nope"), []byte("x")); err == nil {
		t.Error("a missing target must fail")
	}
}
