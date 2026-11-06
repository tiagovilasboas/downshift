// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// treeKey hashes everything a check result depends on: the base commit, the
// tracked diff against it, the name and content of every untracked file, and
// the Go toolchain. Any edit changes it.
func treeKey(base string, untracked []string) (string, error) {
	diff, err := gitOut("diff", base)
	if err != nil {
		return "", err
	}
	goVersion, _ := combined(nil, "go", "version")
	h := sha256.New()
	h.Write([]byte(base + "\x00" + diff + "\x00" + string(goVersion)))
	for _, f := range untracked {
		data, _ := os.ReadFile(f)
		h.Write([]byte("\x00" + f + "\x00"))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// stampPath is where the last green key of this checkout is kept, in the OS
// cache directory (never in the repository), one file per checkout.
func stampPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	top, err := gitOut("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	id := sha256.Sum256([]byte(strings.TrimSpace(top)))
	return filepath.Join(dir, "downshift", "devcheck", hex.EncodeToString(id[:8])+".green"), nil
}

// isGreen reports whether key is the one recorded by the last green run.
func isGreen(path, key string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(data)) == key
}

// recordGreen stores key; a failure to write only costs the shortcut.
func recordGreen(path, key string) {
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, []byte(key+"\n"), 0o644)
	}
}
