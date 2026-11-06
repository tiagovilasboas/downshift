// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

func TestFamilyOf_FileReads(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		// allowed: one regular file, a plain read
		{"cat main.go", FamilyFileRead},
		{"cat internal/core/route.go", FamilyFileRead},
		{"head main.go", FamilyFileRead},
		{"head -n 40 main.go", FamilyFileRead},
		{"head -n40 main.go", FamilyFileRead},
		{"head -40 main.go", FamilyFileRead},
		{"tail main.go", FamilyFileRead},
		{"tail -n 40 main.go", FamilyFileRead},
		{"tail -n40 main.go", FamilyFileRead},
		{"tail -40 main.go", FamilyFileRead},
		{"sed -n 10,20p main.go", FamilyFileRead},
		{"sed -n 10p main.go", FamilyFileRead},
		{"wc -l main.go", FamilyWc},
		{"wc -l a.go b.go c.go", FamilyWc},

		// refused: follow, byte counts, signs, zero and huge counts
		{"tail -f app.log", FamilyNone},
		{"tail -F app.log", FamilyNone},
		{"tail -f -n 5 app.log", FamilyNone},
		{"tail -n 5 -f app.log", FamilyNone},
		{"tail --follow app.log", FamilyNone},
		{"tail -c 100 main.go", FamilyNone},
		{"tail -n +5 main.go", FamilyNone},
		{"head -n -5 main.go", FamilyNone},
		{"head -n 0 main.go", FamilyNone},
		{"head -n 99999999 main.go", FamilyNone},
		{"head -n x main.go", FamilyNone},
		{"head -q main.go", FamilyNone},
		{"head a.go b.go", FamilyNone},
		{"tail -n 5 a.go b.go", FamilyNone},

		// refused: cat options, several files, no file, standard input
		{"cat", FamilyNone},
		{"cat -n main.go", FamilyNone},
		{"cat -A main.go", FamilyNone},
		{"cat a.go b.go", FamilyNone},
		{"cat -", FamilyNone},
		{"head", FamilyNone},
		{"head -n 5", FamilyNone},
		{"head -n 5 -", FamilyNone},
		{"tail -", FamilyNone},

		// refused: sed anything but print of a range
		{"sed -i s/a/b/ main.go", FamilyNone},
		{"sed -n -i 1p main.go", FamilyNone},
		{"sed -i.bak -n 1,5p main.go", FamilyNone},
		{"sed --in-place -n 1,5p main.go", FamilyNone},
		{"sed -n 1,5w out.txt main.go", FamilyNone},
		{"sed -n -e 1,5p main.go", FamilyNone},
		{"sed -n -f script.sed main.go", FamilyNone},
		{"sed -n -s 1,5p main.go", FamilyNone},
		{"sed 1,5p main.go", FamilyNone},
		{"sed -n 5,1p main.go", FamilyNone},
		{"sed -n 0,5p main.go", FamilyNone},
		{"sed -n $p main.go", FamilyNone},
		{"sed -n 1,5pp main.go", FamilyNone},
		{"sed -n 1,5p a.go b.go", FamilyNone},
		{"sed -n 1,5p", FamilyNone},
		{"sed -n 1,5e main.go", FamilyNone},

		// refused: wc options and standard input
		{"wc", FamilyNone},
		{"wc -l", FamilyNone},
		{"wc main.go", FamilyNone},
		{"wc -c main.go", FamilyNone},
		{"wc -lc main.go", FamilyNone},
		{"wc -l -", FamilyNone},
		{"wc -l --files0-from=list.txt", FamilyNone},

		// refused: devices, process and kernel trees
		{"cat /dev/zero", FamilyNone},
		{"cat /dev/urandom", FamilyNone},
		{"head -n 5 /dev/random", FamilyNone},
		{"cat /proc/self/environ", FamilyNone},
		{"cat /proc/kcore", FamilyNone},
		{"cat /sys/kernel/notes", FamilyNone},
		{"wc -l /dev/zero", FamilyNone},
		{"wc -l a.go /dev/stdin", FamilyNone},
		{"cat /dev", FamilyNone},
	}
	for _, tc := range cases {
		if got := FamilyOf(strings.Fields(tc.command)); got != tc.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
}

// The shell metacharacters are refused by the existing gate before a family is
// asked: a redirect, a pipe or a glob never reaches the file-read policy.
func TestDecide_FileReadMetacharactersRefused(t *testing.T) {
	const exe = "/usr/local/bin/downshift"
	for _, cmd := range []string{
		"cat main.go > out.txt", "cat main.go >> out.txt", "cat < main.go", "cat *.go",
		"tail -f app.log", "tail main.go | grep x", "cat $(echo main.go)",
		"cat ~/.ssh/id_rsa", "cat main.go &", "sed -n 1,5p main.go > x",
	} {
		if d := Decide(cmd, exe); d.Rewrite && d.Family == FamilyFileRead {
			t.Errorf("Decide(%q) rewrote a file read: %+v", cmd, d)
		}
	}
	if d := Decide("sed -n '10,20p' main.go", exe); !d.Rewrite || d.Family != FamilyFileRead {
		t.Errorf("quoted sed range not wrapped: %+v", d)
	}
}

func TestSecretPath(t *testing.T) {
	secret := []string{
		".env", ".env.local", "config/.env.production", "prod.env", "server.pem", "tls/server.key",
		"id_rsa", "~/.ssh/config", "api_key.go", "keys.json", "secrets.yaml", "credentials", "aws/credentials",
		".aws/config", "db_password.txt", "token.txt", "private.go", "x.tfvars", "terraform.tfstate",
		".npmrc", "store.jks", "secrets/app.go", "monkey.go",
	}
	plain := []string{"main.go", "internal/core/route.go", "README.md", "docs/guide.md", "src/app.ts", "environment.go"}
	for _, p := range secret {
		if !secretPath(p) {
			t.Errorf("secretPath(%q) = false, want true", p)
		}
	}
	for _, p := range plain {
		if secretPath(p) {
			t.Errorf("secretPath(%q) = true, want false", p)
		}
	}
}

func TestReadCompactible_RegularFilesOnly(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	secret := filepath.Join(dir, ".env")
	for _, f := range []string{file, secret} {
		if err := os.WriteFile(f, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name   string
		family Family
		argv   []string
		want   bool
	}{
		{"regular file", FamilyFileRead, []string{"cat", file}, true},
		{"head of a regular file", FamilyFileRead, []string{"head", "-n", "5", file}, true},
		{"secret file", FamilyFileRead, []string{"cat", secret}, false},
		{"directory", FamilyFileRead, []string{"cat", dir}, false},
		{"missing", FamilyFileRead, []string{"cat", filepath.Join(dir, "nope.go")}, false},
		{"wc of regular files", FamilyWc, []string{"wc", "-l", file, secret}, true},
		{"wc with a directory", FamilyWc, []string{"wc", "-l", file, dir}, false},
		{"another family", FamilyLs, []string{"ls", dir}, true},
	}
	for _, tc := range cases {
		if got := readCompactible(tc.family, tc.argv); got != tc.want {
			t.Errorf("%s: readCompactible = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFileReadHint(t *testing.T) {
	cases := []struct {
		argv []string
		want compressor.CommandHint
	}{
		{[]string{"cat", "a.go"}, compressor.CommandHint{Name: "a.go", First: 1}},
		{[]string{"head", "-n", "400", "a.go"}, compressor.CommandHint{Name: "a.go", First: 1}},
		{[]string{"tail", "-400", "a.go"}, compressor.CommandHint{Name: "a.go", First: 1, Tail: true}},
		{[]string{"sed", "-n", "100,500p", "a.go"}, compressor.CommandHint{Name: "a.go", First: 100}},
		{[]string{"cat", "my file.go"}, compressor.CommandHint{Name: "'my file.go'", First: 1}},
	}
	for _, tc := range cases {
		got := fileReadHint(tc.argv)
		if got.Name != tc.want.Name || got.First != tc.want.First || got.Tail != tc.want.Tail {
			t.Errorf("fileReadHint(%v) = %+v, want %+v", tc.argv, got, tc.want)
		}
	}
}

func TestReadCompactible_RefusesLinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "id_rsa")
	if err := os.WriteFile(target, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "notes.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if readCompactible(FamilyFileRead, []string{"cat", link}) {
		t.Error("a link named notes.md to a key file was accepted")
	}
}
