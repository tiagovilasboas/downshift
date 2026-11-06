// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// maxWordLen caps a stored word. Credentials and hashes are long; command
// and subcommand names are not.
const maxWordLen = 24

// unknownCommand stands for a command whose name fails the stored shape
// (a path, an env assignment, a URL). Nothing else is kept for it.
const unknownCommand = "other"

// safeWord reports whether w may be stored: lowercase letters, digits, '-'
// and '_' only, not starting with '-', at most maxWordLen bytes. It excludes
// every character a path, URL, credential, KEY=value pair or branch name
// needs (slash, '=', ':', '@', backslash, '.').
func safeWord(w string) bool {
	if w == "" || len(w) > maxWordLen || w[0] == '-' {
		return false
	}
	for i := 0; i < len(w); i++ {
		c := w[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// commandName returns the lowercase base name of argv[0] (no directory, no
// .exe), or unknownCommand when it fails safeWord.
func commandName(arg string) string {
	arg = strings.ReplaceAll(arg, "\\", "/")
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(arg)), ".exe")
	if !safeWord(name) {
		return unknownCommand
	}
	return name
}

// subcommandTools are the commands whose first argument is a subcommand name
// (git status, go test). For any other command (grep, rg, ls, find) argv[1] is
// a pattern or a path, which can be a secret, so it is never stored.
var subcommandTools = map[string]bool{
	"git": true, "gh": true, "go": true, "cargo": true, "npm": true, "pnpm": true,
	"yarn": true, "bun": true, "uv": true, "poetry": true, "pdm": true, "pip": true,
	"pip3": true, "kubectl": true, "docker": true, "helm": true, "terraform": true,
	"composer": true, "dotnet": true, "mise": true, "asdf": true, "nx": true,
	"turbo": true, "bazel": true, "flutter": true, "dart": true, "gradle": true,
}

// keepsSubcommand reports whether word may be stored as the subcommand of
// command: command takes subcommands and word passes safeWord as written (an
// uppercase word, such as a key id, is refused, not lowercased into shape).
func keepsSubcommand(command, word string) bool {
	return subcommandTools[command] && safeWord(word)
}

// signatureOf builds the stored signature: the command base name and, for a
// command that takes subcommands, argv[1] when it passes keepsSubcommand.
// Every other word is dropped.
func signatureOf(argv []string) Signature {
	if len(argv) == 0 {
		return newSignature(nil)
	}
	words := []string{commandName(argv[0])}
	if len(argv) > 1 && keepsSubcommand(words[0], argv[1]) {
		words = append(words, argv[1])
	}
	return newSignature(words)
}

func newSignature(words []string) Signature {
	h := sha256.Sum256([]byte(strings.Join(words, " ")))
	return Signature{Words: words, Hash: hex.EncodeToString(h[:8])}
}

// cleanSignature rewrites a stored signature to the current shape. ok is
// false when nothing safe remains (no valid command word).
func cleanSignature(s Signature) (Signature, bool) {
	if len(s.Words) == 0 || !safeWord(s.Words[0]) {
		return Signature{}, false
	}
	words := []string{s.Words[0]}
	if len(s.Words) > 1 && keepsSubcommand(s.Words[0], s.Words[1]) {
		words = append(words, s.Words[1])
	}
	return newSignature(words), true
}
