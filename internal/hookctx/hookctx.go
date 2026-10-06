// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package hookctx carries the hook's global deadline to the optional
// external helper commands (DOWNSHIFT_MINILM_EMBED, DOWNSHIFT_GRAPHIFY_CMD)
// that run deep inside core.Route, and runs those commands with bounded
// time and output. A slow, hung or chatty helper can never stall a spawn:
// it is killed at the deadline and the caller falls back to regex routing.
package hookctx

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// HookBudget is the time a hook may spend classifying after it has read
// its payload, including every external helper call.
const HookBudget = 1500 * time.Millisecond

// MaxCommandOutput caps the stdout read from one helper command.
const MaxCommandOutput = 1 << 20

// waitDelay bounds how long Wait lingers on a killed command whose children
// still hold its stdout open.
const waitDelay = 100 * time.Millisecond

var (
	mu      sync.RWMutex
	current = context.Background()
)

// Set installs ctx as the hook's global context and returns a function that
// restores the previous one.
func Set(ctx context.Context) (restore func()) {
	mu.Lock()
	prev := current
	current = ctx
	mu.Unlock()
	return func() {
		mu.Lock()
		current = prev
		mu.Unlock()
	}
}

// Context returns the hook's global context (Background outside a hook).
func Context() context.Context {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// ErrOutputTooLarge reports a helper that wrote more than MaxCommandOutput.
var ErrOutputTooLarge = errors.New("helper output exceeds limit")

type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.buf.Len()+len(p) > l.max {
		return 0, ErrOutputTooLarge
	}
	return l.buf.Write(p)
}

// RunCommand runs cmdline (split on whitespace, no shell) with stdin, killed
// after timeout or at the hook deadline, whichever comes first. Stdout is
// capped at MaxCommandOutput; stderr is discarded.
func RunCommand(cmdline, stdin string, timeout time.Duration) ([]byte, error) {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return nil, errors.New("empty command")
	}
	ctx, cancel := context.WithTimeout(Context(), timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(stdin)
	out := &limitedBuffer{max: MaxCommandOutput}
	cmd.Stdout = out
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	return out.buf.Bytes(), nil
}
