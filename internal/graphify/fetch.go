// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package graphify

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookctx"
)

const graphifyCmdTimeout = 3 * time.Second

// CmdFetcher runs an external command that reads a node label on stdin and
// prints a NodeInfo JSON object on stdout. Missing command, timeout, or bad
// JSON means Found=false — the classifier stays on regex.
type CmdFetcher struct {
	Cmd string
}

func (c CmdFetcher) FetchNode(label string) NodeInfo {
	if strings.TrimSpace(c.Cmd) == "" || label == "" {
		return NodeInfo{Found: false}
	}
	// Bounded by graphifyCmdTimeout and the hook's global deadline; output
	// is capped. Once the deadline passes, later labels return at once.
	out, err := hookctx.RunCommand(c.Cmd, label, graphifyCmdTimeout)
	if err != nil {
		return NodeInfo{Found: false}
	}
	var info NodeInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return NodeInfo{Found: false}
	}
	return info
}

// FetcherFromEnv returns a command fetcher when DOWNSHIFT_GRAPHIFY_CMD is set.
// DOWNSHIFT_GRAPHIFY=0 disables it. Nil means offline (no graph lookup).
func FetcherFromEnv() GraphFetcher {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DOWNSHIFT_GRAPHIFY")))
	switch v {
	case "0", "false", "off", "no":
		return nil
	}
	cmd := strings.TrimSpace(os.Getenv("DOWNSHIFT_GRAPHIFY_CMD"))
	if cmd == "" {
		return nil
	}
	return CmdFetcher{Cmd: cmd}
}
