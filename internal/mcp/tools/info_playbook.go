// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/playbook"
)

const playbookSchema = `{"type":"object","properties":{"name":{"type":"string","description":"Playbook name. Omit to list the names."}},"additionalProperties":false}`

// generalID is the shipped set that always applies to a delegation.
const generalID = "general"

type playbookEntry struct {
	Name  string `json:"name"`
	Chars int    `json:"chars"`
}

type playbookList struct {
	Playbooks []playbookEntry `json:"playbooks"`
}

type playbookText struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Text    string `json:"text"`
	General string `json:"general"`
}

func (d Deps) playbookTool() Tool {
	return d.build("playbook",
		"Shows the working rules Downshift hands to delegated workers. Without a name it lists the playbooks and their sizes; with a name it returns that playbook's text and the name of General, which always applies. Call it to choose the playbook for a delegation.",
		playbookSchema, []string{"name"},
		func(_ context.Context, a map[string]string) (outcome, error) {
			sets, err := d.sets()
			if err != nil {
				return outcome{}, err
			}
			hs := d.harnessIDs()
			name := strings.TrimSpace(a["name"])
			if name == "" {
				l := playbookList{Playbooks: make([]playbookEntry, 0, len(sets))}
				for _, s := range sets {
					l.Playbooks = append(l.Playbooks, playbookEntry{Name: s.Name, Chars: s.Chars()})
				}
				return outcome{data: l, harnesses: hs, shrink: shrinkPlaybookList}, nil
			}
			s, ok := playbook.Find(sets, name)
			if !ok {
				return outcome{}, fmt.Errorf("unknown playbook; valid names: %s", fieldList(playbook.Names(sets)))
			}
			t := playbookText{Name: s.Name, Source: s.Source.String(), Text: s.Text, General: generalName(sets)}
			return outcome{data: t, harnesses: hs, shrink: shrinkPlaybookText}, nil
		})
}

// sets reads the active playbooks through the port.
func (d Deps) sets() ([]playbook.Set, error) {
	if d.Playbooks == nil {
		return nil, errors.New("the shipped playbooks cannot be read")
	}
	sets, err := d.Playbooks.Sets()
	if err != nil {
		return nil, errors.New("the playbooks cannot be read")
	}
	return sets, nil
}

func generalName(sets []playbook.Set) string {
	for _, s := range sets {
		if s.ID == generalID {
			return s.Name
		}
	}
	return ""
}

func shrinkPlaybookList(data any) (any, bool) {
	l, ok := data.(playbookList)
	if !ok {
		return nil, false
	}
	var cut bool
	l.Playbooks, cut = halve(l.Playbooks)
	return l, cut
}

// shrinkPlaybookText cuts the text at a line boundary.
func shrinkPlaybookText(data any) (any, bool) {
	t, ok := data.(playbookText)
	if !ok || len(t.Text) == 0 {
		return nil, false
	}
	half := t.Text[:len(t.Text)/2]
	if i := strings.LastIndexByte(half, '\n'); i > 0 {
		half = half[:i]
	}
	t.Text = strings.ToValidUTF8(half, "")
	return t, true
}
