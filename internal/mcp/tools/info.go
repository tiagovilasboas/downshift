// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tiagovilasboas/downshift/internal/playbook"
	"github.com/tiagovilasboas/downshift/internal/workers"
)

// Model is one model detected for a harness. The JSON tags mirror what
// `downshift models available --json` prints.
type Model struct {
	ID       string   `json:"id"`
	Tier     string   `json:"tier"`
	Efforts  []string `json:"efforts,omitempty"`
	Source   string   `json:"source"`
	Untiered bool     `json:"untiered"`
}

// ModelList is the detected models of one harness.
type ModelList struct {
	Harness string  `json:"harness"`
	Models  []Model `json:"models"`
	// Fallback is true when the list is only the catalog stand-in.
	Fallback       bool      `json:"fallback"`
	CacheWrittenAt time.Time `json:"cache_written_at,omitzero"`
}

// Models answers which models a harness offers.
type Models interface {
	Available(ctx context.Context, harness string) (ModelList, error)
}

// Playbooks answers the active playbook sets.
type Playbooks interface {
	Sets() ([]playbook.Set, error)
}

// StrainEntry is the fullest quota window of one provider and its headroom
// name ("ok", "strained", "critical" or "unknown"). It is advice only.
type StrainEntry struct {
	Provider    string  `json:"provider"`
	Window      string  `json:"window,omitempty"`
	UsedPct     float64 `json:"used_pct"`
	RateLimited bool    `json:"rate_limited"`
	Headroom    string  `json:"headroom"`
}

// Strain reads the quota strain at call time. The ids it receives are the
// harness ids relevant to the call; the adapter maps them to providers.
type Strain interface {
	Strain(ctx context.Context, ids []string) ([]StrainEntry, error)
}

// ApplyRequest is one spawn the orchestrator asks about. It has no task text.
type ApplyRequest struct {
	Harness   string
	Model     string
	Effort    string
	Tier      string
	AgentType string
	Dir       string
}

// ApplyOutcome is what Downshift would apply for the spawn.
type ApplyOutcome struct {
	Harness        string         `json:"harness"`
	RequestedModel string         `json:"requested_model,omitempty"`
	SelectedModel  string         `json:"selected_model,omitempty"`
	Tier           string         `json:"tier,omitempty"`
	Effort         string         `json:"effort,omitempty"`
	Verdict        string         `json:"verdict"`
	Reason         string         `json:"reason"`
	Applied        bool           `json:"applied"`
	Note           map[string]any `json:"note,omitempty"`
}

// Applier answers what Downshift would apply for a spawn.
type Applier interface {
	Apply(ctx context.Context, in ApplyRequest) (ApplyOutcome, error)
}

// Deps are the ports the information tools read. A nil port makes its tool
// answer with a clear failure; a nil Strain only drops the strain advice.
type Deps struct {
	Models    Models
	Playbooks Playbooks
	Strain    Strain
	Applier   Applier
	// Reporter stores model ids the caller reports; nil means unavailable.
	Reporter Reporter
	// Work starts and reads workers; nil means the tools say they are unavailable.
	Work *workers.Queue
	// WriteEnabled is the owner's switch for mode write; off refuses it before anything starts.
	WriteEnabled bool
	// Root is the resolved project directory the server was started in. delegate
	// uses it as the default dir when the caller sends none.
	Root string
	// Broad is true when Root is a place a containment check cannot protect (the
	// filesystem root or the user's home). delegate refuses a missing dir then,
	// so a worker never defaults into a broad directory.
	Broad bool
	// Harnesses lists the valid harness ids, for argument errors.
	Harnesses func() []string
	Version   string
}

// serverName is the name the tools report for the server.
const serverName = "downshift"

// Info returns the four information tools: providers, playbook, status, apply.
func Info(d Deps) []Tool {
	return []Tool{
		// providers is not read-only: a models list the orchestrator passes is stored as a one-hour hint.
		// providers is the entry point the instructions name, so it is the one tool a deferring client loads up front.
		d.providersTool().hinted("Models this harness offers now", false).alwaysLoaded(),
		d.playbookTool().hinted("Working rules for a worker", true),
		d.statusTool().hinted("Quota advice", true),
		d.applyTool().hinted("What would be applied for a choice", true),
	}
}

func (t Tool) alwaysLoaded() Tool { t.AlwaysLoad = true; return t }

// hinted sets the client-facing hints. Every information tool is idempotent.
func (t Tool) hinted(title string, readOnly bool) Tool {
	t.Title, t.ReadOnly, t.Idempotent = title, readOnly, true
	return t
}

// run is what a tool does once its arguments are parsed.
type run func(ctx context.Context, args map[string]string) (outcome, error)

// outcome is the data a tool returns, the harness ids its strain block covers
// and how to shrink the data when it is too large.
type outcome struct {
	data      any
	harnesses []string
	shrink    shrinkFunc
	next      string
}

// build wraps run with argument parsing, the strain block and the envelope.
func (d Deps) build(name, desc, schema string, fields []string, fn run) Tool {
	return Tool{
		Name: name, Description: desc, InputSchema: json.RawMessage(schema),
		Handler: func(ctx context.Context, raw json.RawMessage) (Result, error) {
			args, err := parseArgs(raw, fields)
			if err != nil {
				return Result{}, err
			}
			out, err := fn(ctx, args)
			if err != nil {
				return Result{}, err
			}
			return d.envelope(ctx, out)
		},
	}
}
