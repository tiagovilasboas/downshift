// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"strconv"
	"strings"
)

// The orchestration token cap: the most tokens the workers of one MCP session
// may spend before delegate refuses to start another wave. Precedence, highest
// first: the user's override (DOWNSHIFT_BUDGET_TOKENS), the policy file
// ("orchestration": {"budget_tokens": N}), none. Zero means no cap.
const BudgetTokensEnv = "DOWNSHIFT_BUDGET_TOKENS"

// ResolveBudgetTokens applies the precedence. A zero value means "not set".
func ResolveBudgetTokens(user, file int64) int64 {
	if user > 0 {
		return user
	}
	return file
}

func parseBudgetTokens(raw string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return n, err == nil && n > 0
}

// EnvBudgetTokens reads the user's override. An invalid value is no override
// and returns a note.
func EnvBudgetTokens(getenv func(string) string) (int64, []string) {
	raw := strings.TrimSpace(getenv(BudgetTokensEnv))
	if raw == "" {
		return 0, nil
	}
	n, ok := parseBudgetTokens(raw)
	if !ok {
		return 0, []string{fmt.Sprintf("%s=%q ignored: want a whole number of tokens above 0", BudgetTokensEnv, raw)}
	}
	return n, nil
}

// FilesBudgetTokens reads orchestration.budget_tokens from the project file,
// then the user file. An invalid value is skipped with a note and the next
// file is tried; an absent or unreadable file holds no value.
func FilesBudgetTokens(loc Locations) (int64, []string) {
	var notes []string
	for _, path := range []string{loc.Project, loc.User} {
		raw, note := orchestrationField(path, "budget_tokens")
		if note != "" {
			notes = append(notes, note)
		}
		if raw == "" {
			continue
		}
		if n, ok := parseBudgetTokens(raw); ok {
			return n, notes
		}
		notes = append(notes, fmt.Sprintf("%s: orchestration.budget_tokens %s ignored: want a whole number of tokens above 0", path, raw))
	}
	return 0, notes
}
