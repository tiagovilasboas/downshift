// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

// frontmatterEnd returns the index of the line that closes the YAML frontmatter
// of an agent file, or -1 when the file has none.
func frontmatterEnd(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return -1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i
		}
	}
	return -1
}

func hasFrontmatter(content string) bool {
	return frontmatterEnd(strings.Split(content, "\n")) >= 0
}

// renderAgent returns the agent file for a profile. A file that already has
// frontmatter keeps everything but its model line. Otherwise the file is built
// with the name, the description and the model; the body is the existing text,
// or the description when there is none, so the agent never has an empty prompt.
// description is not invented: the caller refuses to render without one.
func renderAgent(content, name, modelID, description string) string {
	lines := strings.Split(content, "\n")
	if end := frontmatterEnd(lines); end >= 0 {
		return setModelLine(lines, end, modelID)
	}
	body := strings.TrimSpace(content)
	if body == "" {
		body = description
	}
	return "---\nname: " + name + "\ndescription: " + yamlQuoted(description) + "\nmodel: " + modelID + "\n---\n\n" + body + "\n"
}

// setModelLine replaces the model line inside the frontmatter that closes at
// index end, or adds one just before the closing line.
func setModelLine(lines []string, end int, modelID string) string {
	for j := 1; j < end; j++ {
		if strings.HasPrefix(strings.TrimSpace(lines[j]), "model:") {
			lines[j] = "model: " + modelID
			return strings.Join(lines, "\n")
		}
	}
	lines = append(lines[:end], append([]string{"model: " + modelID}, lines[end:]...)...)
	return strings.Join(lines, "\n")
}

// yamlQuoted writes s as a YAML double-quoted scalar. A JSON string is one, so a
// colon, a quote, a hash or a newline in a description cannot break the frontmatter.
func yamlQuoted(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return strings.TrimSuffix(b.String(), "\n")
}
