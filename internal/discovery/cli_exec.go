// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(io.LimitReader(f, maxOutput)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func runCLI(ctx context.Context, bin string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("%s not installed", bin)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = cliEnv(os.Environ(), bin)
	cmd.WaitDelay = 2 * time.Second // a child that keeps stdout open must not outlive the timeout
	var stdout bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, n: maxOutput}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.w.Len()+len(p) > l.n {
		return 0, errors.New("output too large")
	}
	return l.w.Write(p)
}

// cliEnvAllow is every variable a listing CLI gets besides its own prefix.
// Provider API keys (ANTHROPIC_, OPENAI_, XAI_...) are deliberately absent:
// `kiro-cli`, `grok` and the Cursor CLI list models from their own stored
// sign-in, and handing them the keys of other products only widens what a
// compromised binary on PATH could read.
var cliEnvAllow = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true, "TMPDIR": true,
	"LANG": true, "LC_ALL": true, "LC_CTYPE": true, "TERM": true,
	"XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_CACHE_HOME": true, "XDG_STATE_HOME": true, "XDG_RUNTIME_DIR": true,
	"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
	"HTTPS_PROXY": true, "HTTP_PROXY": true, "NO_PROXY": true, "https_proxy": true, "http_proxy": true, "no_proxy": true,
	"SystemRoot": true, "APPDATA": true, "LOCALAPPDATA": true, "USERPROFILE": true,
}

// cliOwnPrefix is the variable prefix a given CLI may read for itself.
func cliOwnPrefix(bin string) string {
	switch strings.TrimSuffix(filepath.Base(bin), ".exe") {
	case "kiro-cli", "kiro-cli-chat":
		return "KIRO_"
	case "cursor-agent", "agent":
		return "CURSOR_"
	}
	return ""
}

// cliEnv filters environ down to the explicit allowlist plus the CLI's own prefix.
func cliEnv(environ []string, bin string) []string {
	prefix := cliOwnPrefix(bin)
	out := make([]string, 0, len(cliEnvAllow))
	for _, kv := range environ {
		name, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if cliEnvAllow[name] || (prefix != "" && strings.HasPrefix(name, prefix)) {
			out = append(out, kv)
		}
	}
	return out
}
