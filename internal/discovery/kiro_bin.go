// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"os"
	"path/filepath"
)

// EnvKiroBin is the override the KiroCrew docs name (KIROCREW_KIRO_BIN): an
// absolute path to a kiro-cli that the owner wants used.
const EnvKiroBin = "KIROCREW_KIRO_BIN"

// resolveKiroCLI picks the kiro-cli to list models with, in the order the
// KiroCrew install docs give: the override, then the copy the desktop app
// ships, then whatever is on PATH. A candidate counts only if it exists.
func resolveKiroCLI(getenv func(string) string, bundled string, exists func(string) bool) string {
	if p := getenv(EnvKiroBin); p != "" && filepath.IsAbs(p) && exists(p) {
		return p
	}
	if bundled != "" && exists(bundled) {
		return bundled
	}
	return "kiro-cli"
}

// defaultKiroCLI resolves against the real environment and filesystem.
func defaultKiroCLI() string {
	return resolveKiroCLI(os.Getenv, bundledKiroCLI(), fileExists)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
