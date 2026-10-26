// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EnvKiroBin is the override the KiroCrew docs name (KIROCREW_KIRO_BIN): an
// absolute path to a kiro-cli that the owner wants used.
const EnvKiroBin = "KIROCREW_KIRO_BIN"

// errKiroNotSignedIn is returned, without running the list command, when the
// chosen kiro-cli does not confirm a session. The caller keeps its cache.
var errKiroNotSignedIn = errors.New("kiro-cli: not signed in, model list skipped")

// kiroWhoamiTimeout bounds the sign-in probe. It is a variable so a test can
// shorten it; production code never changes it.
var kiroWhoamiTimeout = 5 * time.Second

// resolveKiroCLI picks the kiro-cli to list models with, in the order the
// KiroCrew install docs give: the override, then the copy the desktop app
// ships (only when it reports a sign-in), then the PATH name. signedIn is
// called only for the bundled copy, and only when it exists, so the list
// command never runs on a copy that would open a sign-in page.
func resolveKiroCLI(getenv func(string) string, bundled string, exists func(string) bool, signedIn func(bin string) bool) string {
	if p := getenv(EnvKiroBin); filepath.IsAbs(p) && exists(p) {
		return p
	}
	if bundled != "" && exists(bundled) && signedIn(bundled) {
		return bundled
	}
	return "kiro-cli"
}

// defaultKiroCLI resolves against the real environment and filesystem.
func defaultKiroCLI() string {
	return resolveKiroCLI(os.Getenv, bundledKiroCLI(), fileExists, signedInKiroCLI)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// signedInKiroCLI reports whether `<bin> whoami` says the owner is signed in.
// Any failure, timeout or output without "Logged in" counts as not signed in.
// whoami does not open a browser, unlike the list command it guards.
func signedInKiroCLI(bin string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), kiroWhoamiTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "whoami")
	cmd.Env = cliEnv(os.Environ(), bin)
	cmd.WaitDelay = 2 * time.Second
	var stdout bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, n: maxOutput}
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.Contains(stdout.String(), "Logged in")
}
