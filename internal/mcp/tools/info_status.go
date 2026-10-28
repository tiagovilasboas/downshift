// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import "context"

const statusSchema = `{"type":"object","properties":{},"additionalProperties":false}`

type statusData struct {
	Server               string `json:"server"`
	Version              string `json:"version"`
	Playbooks            int    `json:"playbooks"`
	PlaybooksUnavailable bool   `json:"playbooks_unavailable,omitempty"`
}

func (d Deps) statusTool() Tool {
	return d.build("status",
		"Reports the Downshift server version, how many playbooks are active, and the quota strain of every harness. Call it to check that Downshift is reachable and how much headroom each provider has before fanning out work.",
		statusSchema, nil,
		func(context.Context, map[string]string) (outcome, error) {
			s := statusData{Server: serverName, Version: d.Version}
			if sets, err := d.sets(); err != nil {
				s.PlaybooksUnavailable = true
			} else {
				s.Playbooks = len(sets)
			}
			return outcome{data: s, harnesses: d.harnessIDs()}, nil
		})
}
