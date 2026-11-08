// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// Summary says, once the files are written, what now applies to the
// developer. It reads the written Claude Code document so it never claims a
// session policy that is not in the file.
func Summary(plans []Plan) string {
	wrote := false
	var policy []string
	for _, p := range plans {
		if p.Err != nil || !p.Changed() {
			continue
		}
		wrote = true
		if p.Target.Name != "claude-code" {
			continue
		}
		doc, err := configfile.ParseObject(p.After)
		if err != nil {
			continue
		}
		if m, _ := doc.Get("model"); m == "opusplan" {
			policy = append(policy, "the main session uses opusplan (Opus plans, Sonnet executes)")
		}
		if perms, ok := doc.Get("permissions"); ok {
			if po, isObj := configfile.AsObject(perms); isObj {
				if mode, _ := po.Get("defaultMode"); mode == "plan" {
					policy = append(policy, "sessions start in plan mode: Claude plans and you approve before it edits")
				}
			}
		}
		if ep, ok := doc.Get("enabledPlugins"); ok {
			if po, isObj := configfile.AsObject(ep); isObj {
				if on, _ := po.Get("cc-plugin-you-should-know@builtin"); on == true {
					policy = append(policy, "You should know is on: each check is a billed background request to the session's model (/plugin disable cc-plugin-you-should-know@builtin to turn off)")
				}
			}
		}
	}
	if !wrote {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nWhat changes for you\n")
	b.WriteString("  Subagents: your session chooses each spawn's model; Downshift applies it within your policy and measures it. Your main session is not touched.\n")
	b.WriteString("  Agents: say `downshift full-power` in chat to call the whole Downshift squad on the task at hand, or `downshift shift <squad>` for one domain.\n")
	if len(policy) > 0 {
		fmt.Fprintf(&b, "  Claude Code: %s. Skip this with `downshift setup --no-session-policy`.\n", strings.Join(policy, "; "))
	}
	b.WriteString("  Check anything with `downshift doctor`, see the effect with `downshift stats`.\n\n")
	return b.String()
}

func DiffName(shown string, exists bool, side string) string {
	if !exists {
		return "/dev/null"
	}
	return side + "/" + shown
}

func PrintNotes(w io.Writer, notes []string) {
	for _, n := range notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
}

func Confirm(o Options, prompt string) bool {
	if o.Yes {
		return true
	}
	if o.Out != nil {
		fmt.Fprint(o.Out, prompt)
	}
	if o.In == nil {
		return false
	}
	line, _ := bufio.NewReader(o.In).ReadString('\n')
	if o.Out != nil {
		fmt.Fprintln(o.Out)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
