// dsmon-hook — KiroCrew preToolUse lifecycle tracker
// Writes a line to ~/.harness-downshift/agents.jsonl for every subagent spawn
// so dsmon can show active agents. Always exits 0 (never blocks a spawn).
//
// Build:  go build -o dsmon-hook ./cmd/dsmon-hook
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var spawnTools = map[string]bool{
	"spawn_run":        true,
	"spawn_sub_agents": true,
	"subagent":         true,
	"agent_crew":       true,
	"use_subagent":     true,
}

type hookEvent struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

type agentEvent struct {
	Timestamp string `json:"timestamp"`
	Event     string `json:"event"`      // "spawn"
	Tool      string `json:"tool"`       // e.g. "spawn_run"
	Session   string `json:"session"`    // KIRO_SESSION_ID if available
	Task      string `json:"task"`       // first 80 chars of task/prompt
	Model     string `json:"model"`      // requested model if set
}

func main() {
	// Always exit 0 — we are observers only, never gatekeepers
	defer os.Exit(0)

	var ev hookEvent
	sc := bufio.NewScanner(os.Stdin)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	raw := strings.Join(lines, "")
	if raw == "" {
		return
	}
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return
	}
	if !spawnTools[ev.ToolName] {
		return // not a spawn — exit 0 silently
	}

	var ti map[string]any
	json.Unmarshal(ev.ToolInput, &ti) //nolint:errcheck

	task := taskText(ti)
	if len([]rune(task)) > 80 {
		task = string([]rune(task)[:80]) + "…"
	}
	model := ""
	if v, ok := ti["model"].(string); ok {
		model = v
	}

	ae := agentEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Event:     "spawn",
		Tool:      ev.ToolName,
		Session:   os.Getenv("KIRO_SESSION_ID"),
		Task:      task,
		Model:     model,
	}

	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".harness-downshift", "agents.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	line, err := json.Marshal(ae)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "%s\n", line)
}

func taskText(m map[string]any) string {
	for _, key := range []string{"task", "prompt", "description"} {
		if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	// tasks array (spawn_sub_agents)
	if tasks, ok := m["tasks"].([]any); ok && len(tasks) > 0 {
		if s, ok := tasks[0].(string); ok {
			return fmt.Sprintf("[%d tasks] %s", len(tasks), s)
		}
	}
	return ""
}
