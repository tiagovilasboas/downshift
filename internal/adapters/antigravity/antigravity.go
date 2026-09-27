package antigravity

import (
	"encoding/json"
	"strings"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/hookutil"
)

const harnessID = "antigravity"

type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type Event struct {
	ToolCall ToolCall `json:"toolCall"`
}

type Output struct {
	Decision  string          `json:"decision"`
	Overwrite json.RawMessage `json:"overwrite,omitempty"`
}

func (ev Event) TaskText() string {
	var ti map[string]any
	if err := json.Unmarshal(ev.ToolCall.Args, &ti); err != nil {
		return ""
	}
	return hookutil.TaskText(ti, "Subagents")
}

func Handle(ev Event, r ...core.Resolver) (Output, string, core.Decision) {
	if !strings.EqualFold(ev.ToolCall.Name, "invoke_subagent") && !strings.EqualFold(ev.ToolCall.Name, "spawn_agent") {
		return allow(), "", core.Decision{}
	}

	var ti map[string]any
	if err := json.Unmarshal(ev.ToolCall.Args, &ti); err != nil {
		return allow(), "", core.Decision{}
	}

	subagentsRaw, ok := ti["Subagents"].([]any)
	if !ok || len(subagentsRaw) == 0 {
		return allow(), "", core.Decision{}
	}

	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}

	changed := false
	var lastDecision core.Decision

	for _, raw := range subagentsRaw {
		subagent, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		prompt := hookutil.StringField(subagent, "Prompt")
		if prompt == "" {
			continue
		}

		currentModel := hookutil.StringField(subagent, "Model")
		decision := core.Route(prompt, harnessID, currentModel, res)
		lastDecision = decision

		var mappedModel string
		switch decision.Tier {
		case core.TierSmall:
			mappedModel = "flash_lite"
		case core.TierMid:
			mappedModel = "flash"
		case core.TierFrontier:
			mappedModel = "pro"
		default:
			mappedModel = "inherit"
		}

		plan := decision.Plan(core.CursorCaps, res)
		if !plan.PreserveExplicit {
			subagent["Model"] = mappedModel
			changed = true
		}
	}

	if !changed {
		return allow(), "", lastDecision
	}

	updatedArgs, err := json.Marshal(ti)
	if err != nil {
		return allow(), "", lastDecision
	}

	return Output{
		Decision:  "allow",
		Overwrite: updatedArgs,
	}, "downshift: mapped to antigravity tier " + lastDecision.Tier.String(), lastDecision
}

func allow() Output {
	return Output{Decision: "allow"}
}
