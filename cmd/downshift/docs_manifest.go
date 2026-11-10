// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/tiagovilasboas/downshift-ai/internal/clispec"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/gateway"
	"github.com/tiagovilasboas/downshift-ai/internal/gateway/ninerouter"
	"github.com/tiagovilasboas/downshift-ai/internal/mcp/tools"
)

// docsManifest is what the documentation is checked against: the commands,
// flags, variables, MCP tools, default endpoints and harness capabilities this
// build really has. The site and the docs guards read it instead of the code.
type docsManifest struct {
	Version   string                              `json:"version"`
	Commands  []manifestCommand                   `json:"commands"`
	EnvVars   []string                            `json:"env_vars"`
	MCPTools  []string                            `json:"mcp_tools"`
	Endpoints map[string]string                   `json:"default_endpoints"`
	Harnesses map[string]core.HarnessCapabilities `json:"harness_capabilities"`
}

type manifestCommand struct {
	Name         string         `json:"name"`
	Summary      string         `json:"summary"`
	Hidden       bool           `json:"hidden,omitempty"`
	AliasOf      string         `json:"alias_of,omitempty"`
	Legacy       bool           `json:"legacy,omitempty"`
	Experimental bool           `json:"experimental,omitempty"`
	Flags        []clispec.Flag `json:"flags,omitempty"`
}

// buildDocsManifest gathers the manifest from the shared registries. Slices
// are sorted and maps marshal in key order, so the bytes are deterministic.
func buildDocsManifest() docsManifest {
	m := docsManifest{
		Version:   buildVersion,
		EnvVars:   clispec.EnvVars(),
		MCPTools:  mcpToolNames(),
		Endpoints: map[string]string{gateway.SupportedName: ninerouter.DefaultEndpoint},
		Harnesses: map[string]core.HarnessCapabilities{},
	}
	for _, c := range clispec.Commands() {
		m.Commands = append(m.Commands, manifestCommand{Name: c.Name, Summary: c.Summary, Hidden: c.Hidden, AliasOf: c.AliasOf, Legacy: c.Legacy, Experimental: c.Experimental, Flags: c.Flags})
	}
	for _, h := range core.AllHarnessCaps() {
		m.Harnesses[h.Harness] = h.Caps
	}
	return m
}

// mcpToolNames lists the tools the MCP server registers, read from the same
// constructors serveMcp uses. A zero Deps is enough to name them.
func mcpToolNames() []string {
	var names []string
	for _, t := range append(tools.Info(tools.Deps{}), tools.WorkTools(tools.Deps{})...) {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

// runDocsManifest prints the manifest as indented JSON.
func runDocsManifest(w io.Writer) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(buildDocsManifest()); err != nil {
		return 1
	}
	return 0
}
