// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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

// CursorCLI uses Cursor's read-only `models` command. It relies on the CLI's
// existing authentication and never reads credentials or starts a login.
// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift
type CursorCLI struct{ Bin string }

func (CursorCLI) Harness() string { return "cursor" }
func (CursorCLI) Name() string    { return "Cursor CLI models" }

func (c CursorCLI) Discover(ctx context.Context) ([]Model, error) {
	bin := c.Bin
	if bin == "" {
		if p, err := exec.LookPath("cursor-agent"); err == nil {
			bin = p
		} else {
			// `agent` is also used by unrelated products. Confirm the command's
			// identity through help before invoking a listing on that executable.
			help, err := runCLI(ctx, "agent", "models", "--help")
			if err != nil || !strings.Contains(string(help), "Usage: agent models") ||
				!strings.Contains(string(help), "List available models for this account") {
				return nil, errors.New("Cursor CLI not installed (cursor-agent or verified agent models)")
			}
			bin = "agent"
		}
	}
	out, err := runCLI(ctx, bin, "models")
	if err != nil {
		return nil, fmt.Errorf("Cursor models failed (check existing CLI authentication): %w", err)
	}
	return parseCursorList(out)
}

var cursorANSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
var cursorID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/+\[\],=-]*$`)
var cursorSelection = regexp.MustCompile(` \((?:current|default)(?:, (?:current|default))*\)$`)

// parseCursorList accepts the complete text listing emitted by Cursor CLI.
// The header, footer and every row must match: truncated or changed formats
// are unknown rather than a partial authoritative model set.
func parseCursorList(data []byte) ([]Model, error) {
	sc := bufio.NewScanner(bytes.NewReader(cursorANSI.ReplaceAll(data, nil)))
	inBlock, complete := false, false
	seen := map[string]bool{}
	var out []Model
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !inBlock {
			if line != "Available models" {
				return nil, errors.New("cursor list: missing model header")
			}
			inBlock = true
			continue
		}
		if strings.HasPrefix(line, "Tip: use ") {
			complete = true
			continue
		}
		if complete {
			return nil, errors.New("cursor list: unexpected trailing output")
		}
		row := cursorSelection.ReplaceAllString(line, "")
		id, display, hasDisplay := strings.Cut(row, " - ")
		if !cursorID.MatchString(id) || (hasDisplay && strings.TrimSpace(display) == "") || seen[id] {
			return nil, errors.New("cursor list: malformed or duplicate model row")
		}
		seen[id] = true
		out = append(out, Model{ID: id, Display: display})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("cursor list: %w", err)
	}
	if !complete || len(out) == 0 {
		return nil, errors.New("cursor list: empty or incomplete model listing")
	}
	return out, nil
}
