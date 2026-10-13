// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package scoreindex

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// ModelStatus is one session model and its score from the selected source.
type ModelStatus struct {
	ID      string   `json:"id"`
	Score   *float64 `json:"score,omitempty"`
	Covered bool     `json:"covered"`
}

// HarnessStatus lists the enabled session models of one harness.
type HarnessStatus struct {
	Harness string        `json:"harness"`
	Models  []ModelStatus `json:"models"`
}

// SourceStatus is the cached state and session coverage of one source.
type SourceStatus struct {
	SourceInfo
	Covered int `json:"covered"`
	Total   int `json:"total"`
}

// Status is the `--status` view.
type Status struct {
	Disabled       bool      `json:"disabled"`
	Path           string    `json:"path"`
	Present        bool      `json:"present"`
	Updated        time.Time `json:"updated,omitempty"`
	AgeSeconds     int64     `json:"age_seconds,omitempty"`
	SelectedSource string    `json:"selected_source,omitempty"`
	// Reselect is set when the current session lists would pick another
	// source; the next refresh applies it.
	Reselect  string          `json:"reselect,omitempty"`
	Sources   []SourceStatus  `json:"sources,omitempty"`
	Harnesses []HarnessStatus `json:"harnesses,omitempty"`
}

// BuildStatus reads the cache at path (empty means the state dir) and reports
// coverage for sessions (harness -> enabled session ids).
func BuildStatus(path string, sessions map[string][]string, now time.Time) (*Status, error) {
	st := &Status{Disabled: Disabled()}
	if path == "" {
		p, err := Path()
		if err != nil {
			return nil, err
		}
		path = p
	}
	st.Path = path
	if st.Disabled {
		return st, nil
	}
	f, err := Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, fmt.Errorf("cache unreadable: %w", err)
	}
	st.Present = true
	st.Updated = f.Updated
	st.AgeSeconds = int64(now.Sub(f.Updated).Seconds())
	st.SelectedSource = f.SelectedSource
	if want := SelectSource(f, sessions); want != "" && want != f.SelectedSource {
		st.Reselect = want
	}
	for _, s := range f.Sources {
		c, t := f.Coverage(s.Name, sessions)
		st.Sources = append(st.Sources, SourceStatus{SourceInfo: s, Covered: c, Total: t})
	}
	scores := map[string]float64{}
	for _, r := range f.Rows {
		if r.Source == f.SelectedSource {
			scores[sessionKey(r.Harness, r.ID)] = r.Score
		}
	}
	for _, h := range sortedKeys(sessions) {
		hs := HarnessStatus{Harness: h}
		seen := map[string]bool{}
		for _, id := range sessions[h] {
			if id == "" || id == "inherit" || seen[id] {
				continue
			}
			seen[id] = true
			m := ModelStatus{ID: id}
			if v, ok := scores[sessionKey(h, id)]; ok {
				v := v
				m.Score, m.Covered = &v, true
			}
			hs.Models = append(hs.Models, m)
		}
		st.Harnesses = append(st.Harnesses, hs)
	}
	return st, nil
}

// WriteJSON prints the status as JSON.
func (s *Status) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	return enc.Encode(s)
}

// WriteText prints the human status.
func (s *Status) WriteText(w io.Writer) {
	if s.Disabled {
		fmt.Fprintf(w, "scores: off (%s=off, no network, router ignores scores)\n", EnvSwitch)
		return
	}
	if !s.Present {
		fmt.Fprintf(w, "scores: missing at %s\nrun `downshift models scores --refresh`\n", s.Path)
		return
	}
	age := (time.Duration(s.AgeSeconds) * time.Second).Round(time.Minute)
	fmt.Fprintf(w, "scores: %s\nupdated: %s (%s ago)\nselected source: %s\n",
		s.Path, s.Updated.Format(time.RFC3339), age, orNone(s.SelectedSource))
	if s.Reselect != "" {
		fmt.Fprintf(w, "note: current session lists would select %s; the next refresh applies it\n", s.Reselect)
	}
	fmt.Fprintln(w, "sources (one scale each, never mixed):")
	for _, src := range s.Sources {
		mark := " "
		if src.Name == s.SelectedSource {
			mark = "*"
		}
		fmt.Fprintf(w, " %s %-14s rows %-3d covers %d/%d session models  %s\n", mark, src.Name, src.Rows, src.Covered, src.Total, src.Status)
	}
	fmt.Fprintln(w, "session models:")
	for _, h := range s.Harnesses {
		if len(h.Models) == 0 {
			fmt.Fprintf(w, "  %s: no session list\n", h.Harness)
			continue
		}
		fmt.Fprintf(w, "  %s:\n", h.Harness)
		for _, m := range h.Models {
			if m.Covered {
				fmt.Fprintf(w, "    %-34s %.2f\n", m.ID, *m.Score)
			} else {
				fmt.Fprintf(w, "    %-34s uncovered\n", m.ID)
			}
		}
	}
	fmt.Fprintln(w, "attribution:")
	for _, src := range s.Sources {
		fmt.Fprintf(w, "  %s\n", src.Attribution)
	}
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
