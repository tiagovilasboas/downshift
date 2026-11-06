// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
)

// What the file-read families check when the command runs, not when its argv
// is read: what the operands name on disk. A command that fails these checks
// still runs, with its output untouched and never held or stored.

// readCompactible reports whether the output of a file-read family may be
// compacted: every operand is a regular file and not a link (a link named
// notes.md may point at a key, and the name check reads only the link), and,
// for cat, head, tail and sed,
// not a file that may hold a secret. The operands are the argv words after
// the options, found by the same parser the family used.
func readCompactible(family Family, argv []string) bool {
	var files []string
	switch {
	case family == FamilyWc:
		files = argv[2:]
	case family == FamilyFileRead:
		files = []string{argv[len(argv)-1]}
	default:
		return true
	}
	for _, f := range files {
		info, err := os.Lstat(f)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
		if family == FamilyFileRead && secretPath(f) {
			return false
		}
	}
	return true
}

// secretWords in a file name mark a file that may hold a credential.
var secretWords = []string{"key", "secret", "credential", "token", "passw", "private"}

// secretNames are exact file names (lower case) that hold credentials or
// local state that does.
var secretNames = map[string]bool{
	".npmrc": true, ".netrc": true, ".pypirc": true, ".htpasswd": true,
	"id_rsa": true, "id_dsa": true, "id_ecdsa": true, "id_ed25519": true,
	"terraform.tfstate": true, "kubeconfig": true,
}

// secretDirs are directories whose files are never read through the compactor.
var secretDirs = map[string]bool{".ssh": true, ".aws": true, ".gnupg": true, ".kube": true, ".docker": true}

// secretExts are file extensions of keys, certificates stores and variable files.
var secretExts = []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".asc", ".gpg", ".tfvars"}

// secretPath reports whether p looks like a secret or an environment file:
// ".env" and ".env.*", the extensions in secretExts, the names in
// secretNames, any name that holds a word in secretWords, and any file under a
// directory in secretDirs. It is deliberately broad: a false hit only leaves
// one file raw.
func secretPath(p string) bool {
	slashed := strings.ToLower(filepath.ToSlash(p))
	parts := strings.Split(slashed, "/")
	base := parts[len(parts)-1]
	switch {
	case base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env"):
		return true
	case secretNames[base]:
		return true
	}
	for _, ext := range secretExts {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	for _, w := range secretWords {
		if strings.Contains(base, w) {
			return true
		}
	}
	for _, d := range parts[:len(parts)-1] {
		if secretDirs[d] || strings.Contains(d, "secret") || strings.Contains(d, "credential") {
			return true
		}
	}
	return false
}
