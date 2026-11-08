// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package home

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

// maxIssues caps the issues listed on the home screen; doctor shows them all.
const maxIssues = 5

type check struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

type health struct {
	ok, warn, fail int
	issues         []check // fails first, then warnings
}

// parseHealth reads `doctor --json`. Doctor exits non-zero when a check fails,
// so the exit code is not an error here; output that is not JSON is.
func parseHealth(r command.Result) (health, error) {
	if r.Err != nil {
		return health{}, r.Err
	}
	var doc struct {
		Checks []check `json:"checks"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &doc); err != nil {
		return health{}, fmt.Errorf("doctor --json: %w", err)
	}
	var h health
	var warns []check
	for _, c := range doc.Checks {
		switch c.Status {
		case "ok":
			h.ok++
		case "fail":
			h.fail++
			h.issues = append(h.issues, c)
		case "warn":
			h.warn++
			warns = append(warns, c)
		}
	}
	h.issues = append(h.issues, warns...)
	return h, nil
}

// room is the cells left on a line of width after used cells; 0 means no limit.
func room(width, used int) int {
	if width <= 0 {
		return 0
	}
	return max(width-used, 8)
}

func healthBlock(t theme.Theme, h health, err error, width int) string {
	if err != nil {
		return fmt.Sprintf(" %s health checks did not load: %s\n   %s\n",
			t.Fail.Render(theme.SymFail), err, t.Muted.Render("r try again"))
	}
	var b strings.Builder
	fmt.Fprintf(&b, " %s %d ok   %s %d warnings   %s %d failed\n",
		t.OK.Render(theme.SymOK), h.ok, t.Warn.Render(theme.SymWarn), h.warn,
		t.Fail.Render(theme.SymFail), h.fail)
	for i, c := range h.issues {
		if i == maxIssues {
			fmt.Fprintf(&b, "   %s\n", t.Muted.Render(fmt.Sprintf("+%d more: run doctor from the command list", len(h.issues)-maxIssues)))
			break
		}
		fmt.Fprintf(&b, " %s %s: %s\n", t.Status(c.Status), c.ID, oneLine(c.Detail, room(width, 4+len(c.ID)+2)))
		if c.Fix != "" {
			fmt.Fprintf(&b, "   %s\n", t.Muted.Render(oneLine("fix: "+c.Fix, room(width, 3))))
		}
	}
	return b.String()
}

// oneLine collapses s to a single line of at most limit cells (100 when limit
// is not positive), cutting on cell boundaries so UTF-8 is never split.
func oneLine(s string, limit int) string {
	if limit <= 0 {
		limit = 100
	}
	return frame.Clip(strings.Join(strings.Fields(s), " "), limit)
}
