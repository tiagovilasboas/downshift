// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// codexLine is the slice of a Codex session line this package reads: calls
// and their outputs are response_item payloads joined by call_id.
type codexLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type      string          `json:"type"`
		Name      string          `json:"name"`
		CallID    string          `json:"call_id"`
		Input     string          `json:"input"`
		Arguments string          `json:"arguments"`
		Output    json.RawMessage `json:"output"`
	} `json:"payload"`
}

// codexShellTools are the function-call names that carry one shell command.
var codexShellTools = map[string]bool{
	"shell": true, "exec_command": true, "shell_command": true,
	"container.exec": true, "local_shell": true,
}

var codexItem = []byte(`"response_item"`)

// jsCmdRe finds the cmd string literal of a tools.exec_command({...}) call in
// the JavaScript the current Codex "exec" tool runs. A cmd given by a variable
// cannot be resolved and is not counted.
var jsCmdRe = regexp.MustCompile("exec_command\\(\\{\\s*[\"']?cmd[\"']?\\s*:\\s*(\"(?:\\\\.|[^\"\\\\])*\"|'(?:\\\\.|[^'\\\\])*'|`[^`$]*`)")

// parseCodex reads shell calls from a Codex session: the "exec" tool's
// JavaScript (one or more exec_command calls) and plain shell function calls.
func parseCodex(line []byte) (event, bool) {
	if !bytes.Contains(line, codexItem) {
		return event{}, false
	}
	var l codexLine
	if json.Unmarshal(line, &l) != nil || l.Type != "response_item" || l.Payload.CallID == "" && l.Payload.Name == "" {
		return event{}, false
	}
	p := l.Payload
	ev := event{at: parseTime(l.Timestamp)}
	switch p.Type {
	case "custom_tool_call":
		if p.Name == "exec" {
			ev.calls = jsCalls(p.CallID, p.Input)
		}
	case "function_call":
		if codexShellTools[p.Name] {
			if cmd := argsCommand(p.Arguments); cmd != "" {
				ev.calls = []call{{id: p.CallID, command: cmd}}
			}
		}
	case "custom_tool_call_output", "function_call_output":
		ev.results = []result{{id: p.CallID, bytes: textBytes(p.Output)}}
	}
	return ev, len(ev.calls)+len(ev.results) > 0
}

// jsCalls extracts every literal exec_command cmd from a JavaScript source.
// The calls share one id: their joint output is split between them.
func jsCalls(id, src string) []call {
	var out []call
	for _, m := range jsCmdRe.FindAllStringSubmatch(src, -1) {
		if cmd := jsString(m[1]); cmd != "" {
			out = append(out, call{id: id, command: cmd})
		}
	}
	return out
}

// jsString decodes a quoted JavaScript string literal.
func jsString(lit string) string {
	switch lit[0] {
	case '"':
		if s, err := strconv.Unquote(lit); err == nil {
			return s
		}
	case '\'':
		inner := strings.ReplaceAll(lit[1:len(lit)-1], `\'`, `'`)
		if s, err := strconv.Unquote(`"` + strings.ReplaceAll(inner, `"`, `\"`) + `"`); err == nil {
			return s
		}
	case '`':
		return lit[1 : len(lit)-1]
	}
	return ""
}

// argsCommand reads the command of a shell function call: {"cmd": "..."} or
// {"command": "..."} or {"command": ["bash", "-lc", "..."]}.
func argsCommand(args string) string {
	var a struct {
		Cmd     string          `json:"cmd"`
		Command json.RawMessage `json:"command"`
	}
	if json.Unmarshal([]byte(args), &a) != nil {
		return ""
	}
	if a.Cmd != "" {
		return a.Cmd
	}
	var s string
	if json.Unmarshal(a.Command, &s) == nil {
		return s
	}
	var argv []string
	if json.Unmarshal(a.Command, &argv) != nil || len(argv) == 0 {
		return ""
	}
	if len(argv) >= 3 && strings.HasSuffix(argv[0], "sh") && strings.HasPrefix(argv[1], "-") && strings.HasSuffix(argv[1], "c") {
		return argv[2]
	}
	return strings.Join(argv, " ")
}
