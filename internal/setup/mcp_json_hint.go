// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// mcpInvalidJSONWarning explains why setup left a harness file alone when it
// is not strict JSON (a commented opencode.jsonc, a trailing comma) and says
// what to add by hand. Setup never rewrites such a file: that would drop the
// user's comments.
func mcpInvalidJSONWarning(name, shown string, m Mcp, bin string) string {
	head := fmt.Sprintf("warning: %s: %s: not strict JSON (comments or a trailing comma?); the MCP server was not registered and the file was left untouched\n", name, shown)
	entry, err := configfile.MarshalValue(mcpJSONEntry(m, bin))
	if err != nil {
		return head + "\n"
	}
	path := McpJSONPath(m.Layout)
	label := strings.Join(path, ".") + "." + McpServerName
	return head + fmt.Sprintf("  to register it by hand, add this entry as %s:\n    \"%s\": %s\n\n", label, McpServerName, entry)
}
