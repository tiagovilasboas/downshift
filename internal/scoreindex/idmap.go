// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package scoreindex

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed idmap.json
var embeddedIDMap []byte

// Target is one session model id of one harness.
type Target struct {
	Harness string `json:"harness"`
	ID      string `json:"id"`
}

// Entry maps the public names of one model, per source, to the session ids
// that run it. Names are exact (case-insensitive); there is no prefix or
// family guessing.
type Entry struct {
	Name       string   `json:"name"`
	Epoch      []string `json:"epoch"`
	OpenRouter []string `json:"openrouter"`
	Targets    []Target `json:"ids"`
}

// IDMap is the embedded public-name to session-id table (idmap.json).
type IDMap struct {
	Version int     `json:"version"`
	Models  []Entry `json:"models"`

	once    sync.Once
	byEpoch map[string]int
	byOR    map[string]int
	err     error
}

var effortSuffix = regexp.MustCompile(`_(minimal|low|medium|high|xhigh|max)$`)

// normalizeEpochName lowercases a name and strips an effort suffix such as
// "_high", the way Epoch names run variants (claude-haiku-5-5_high).
func normalizeEpochName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return effortSuffix.ReplaceAllString(s, "")
}

func normalizeORName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ParseIDMap decodes and validates an id map document.
func ParseIDMap(data []byte) (*IDMap, error) {
	var m IDMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("idmap: %w", err)
	}
	if m.Version != 1 {
		return nil, fmt.Errorf("idmap: unsupported version %d", m.Version)
	}
	if err := m.index(); err != nil {
		return nil, err
	}
	return &m, nil
}

// DefaultIDMap returns the embedded map. It panics only if the embedded file
// is invalid, which a unit test rules out.
func DefaultIDMap() *IDMap {
	m, err := ParseIDMap(embeddedIDMap)
	if err != nil {
		panic(err)
	}
	return m
}

func (m *IDMap) index() error {
	m.once.Do(func() {
		m.byEpoch, m.byOR = map[string]int{}, map[string]int{}
		put := func(idx map[string]int, key string, i int, what string) {
			if key == "" {
				m.err = fmt.Errorf("idmap: empty %s name in %q", what, m.Models[i].Name)
				return
			}
			if j, dup := idx[key]; dup && j != i {
				m.err = fmt.Errorf("idmap: %s name %q is in both %q and %q", what, key, m.Models[j].Name, m.Models[i].Name)
				return
			}
			idx[key] = i
		}
		for i, e := range m.Models {
			if e.Name == "" || len(e.Targets) == 0 {
				m.err = fmt.Errorf("idmap: entry %d needs a name and ids", i)
				return
			}
			// Epoch also names runs by the model id, so the entry name is a key.
			put(m.byEpoch, normalizeEpochName(e.Name), i, "epoch")
			for _, n := range e.Epoch {
				put(m.byEpoch, normalizeEpochName(n), i, "epoch")
			}
			for _, n := range e.OpenRouter {
				put(m.byOR, normalizeORName(n), i, "openrouter")
			}
			for _, t := range e.Targets {
				if t.Harness == "" || t.ID == "" {
					m.err = fmt.Errorf("idmap: entry %q has an empty target", e.Name)
					return
				}
			}
		}
	})
	return m.err
}

// LookupEpoch returns the session ids for an Epoch model name or id.
func (m *IDMap) LookupEpoch(name string) []Target {
	if m == nil || m.index() != nil {
		return nil
	}
	if i, ok := m.byEpoch[normalizeEpochName(name)]; ok {
		return m.Models[i].Targets
	}
	return nil
}

// LookupOpenRouter returns the session ids for an OpenRouter model slug. A
// variant slug such as "anthropic/claude-haiku-5.5:batch" is a different
// name and is not mapped.
func (m *IDMap) LookupOpenRouter(slug string) []Target {
	if m == nil || m.index() != nil {
		return nil
	}
	if i, ok := m.byOR[normalizeORName(slug)]; ok {
		return m.Models[i].Targets
	}
	return nil
}
