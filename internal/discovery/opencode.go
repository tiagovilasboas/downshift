// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// OpenCodeCLI uses OpenCode's read-only `models` command, which prints one
// provider/model id per line for the providers the local install is set up
// with. It reads no credentials and starts no login.
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
type OpenCodeCLI struct{ Bin string }

func (OpenCodeCLI) Harness() string { return "opencode" }
func (OpenCodeCLI) Name() string    { return "OpenCode models" }

func (c OpenCodeCLI) Discover(ctx context.Context) ([]Model, error) {
	bin := c.Bin
	if bin == "" {
		p, err := exec.LookPath("opencode")
		if err != nil || !trustedBinaryFn(p) {
			return nil, errors.New("OpenCode not installed (opencode on a trusted path)")
		}
		bin = p
	}
	out, err := runCLI(ctx, bin, "models")
	if err != nil {
		return nil, fmt.Errorf("OpenCode models failed: %w", err)
	}
	return parseOpenCodeList(out)
}

var openCodeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._:/+-]*$`)
var openCodeANSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

// parseOpenCodeList accepts only lines that are provider/model ids. Any other
// line, or a duplicate, makes the whole listing unknown rather than partial.
func parseOpenCodeList(data []byte) ([]Model, error) {
	sc := bufio.NewScanner(bytes.NewReader(openCodeANSI.ReplaceAll(data, nil)))
	seen := map[string]bool{}
	var out []Model
	for sc.Scan() {
		id := strings.TrimSpace(sc.Text())
		if id == "" {
			continue
		}
		if !openCodeID.MatchString(id) || seen[id] {
			return nil, errors.New("opencode list: malformed or duplicate model row")
		}
		seen[id] = true
		out = append(out, Model{ID: id})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("opencode list: %w", err)
	}
	if len(out) == 0 {
		return nil, errors.New("opencode list: no models")
	}
	return out, nil
}
