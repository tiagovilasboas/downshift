// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

// McpServerName is the name setup registers the MCP server under.
const McpServerName = "downshift"

// McpServeArgs are the arguments after the binary that start the server.
var McpServeArgs = []string{"mcp", "serve"}

// The env marker setup adds to the Claude Code entry.
const (
	McpMarkerKey   = "DOWNSHIFT_MANAGED"
	McpMarkerValue = "setup"
)

// McpSameArgs reports whether two argument lists are equal.
func McpSameArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
