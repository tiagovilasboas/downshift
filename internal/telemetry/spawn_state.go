// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This file is the single place that says what Downshift did to one spawn, in
// a form every surface can show: the stderr line, `downshift session`,
// `downshift watch`, the statusline and the live record a panel mod reads. It
// works from an Event, so it serves every harness and reads no prompt text.

// Spawn states. Exactly one applies to a decision.
const (
	// StateSwitched: Downshift changed the spawn's model or effort.
	StateSwitched = "switched"
	// StateHeld: Downshift considered a change and a guardrail, the credit gate
	// or a guessed baseline kept the spawn as it was.
	StateHeld = "held"
	// StateObserved: Downshift looked and the spawn already fit.
	StateObserved = "observed"
	// StatePaused: routing is paused for the session; classified, never changed.
	StatePaused = "paused"
	// StateControl: control group or DOWNSHIFT_NO_ROUTE; classified, never changed.
	StateControl = "control"
	// StateBlocked: a policy-mode harness denied the spawn to be respawned.
	StateBlocked = "blocked"
)

// ActiveWindow is how recent the last spawn must be for Downshift to show as
// active. Hooks only run on a spawn, so "idle" means no recent spawn, not that
// the hook is missing.
const ActiveWindow = 5 * time.Minute

// SpawnStateOf classifies a decision event. Held wins over switched: a hold
// means the change was not applied, whatever the outcome field says.
func SpawnStateOf(ev Event) string {
	switch {
	case ev.Outcome == OutcomeBlocked:
		return StateBlocked
	case hasCode(ev.ReasonCodes, ReasonPaused):
		return StatePaused
	case ev.Outcome == OutcomeBaseline:
		return StateControl
	case isHeld(ev):
		return StateHeld
	case ev.Outcome == OutcomeRewriteEmitted:
		return StateSwitched
	}
	return StateObserved
}

func isHeld(ev Event) bool {
	if len(ev.Corrections) > 0 || ev.CreditHeld {
		return true
	}
	for _, c := range ev.ReasonCodes {
		if strings.HasPrefix(c, "HELD_") {
			return true
		}
	}
	return false
}

// SpawnRecord is the prompt-free record of one spawn: no task text, no file
// path, no raw session or agent id. It is the contract a panel mod reads (see
// docs/reference/spawn-state.md).
type SpawnRecord struct {
	State      string   `json:"state"`
	Harness    string   `json:"harness"`
	Session    string   `json:"session,omitempty"` // first 8 hex of the hashed id
	Time       string   `json:"time"`
	FromModel  string   `json:"from_model"`
	ToModel    string   `json:"to_model"`
	FromEffort string   `json:"from_effort,omitempty"`
	ToEffort   string   `json:"to_effort,omitempty"`
	WantModel  string   `json:"recommended_model,omitempty"` // what a held spawn would have switched to
	Complexity string   `json:"complexity,omitempty"`
	Reasons    []string `json:"reason_codes,omitempty"`
}

// NewSpawnRecord builds the record from a finished decision event (after
// MarkUnchanged, so ToModel is what the child actually runs on).
func NewSpawnRecord(ev Event) SpawnRecord {
	r := SpawnRecord{
		State: SpawnStateOf(ev), Harness: ev.Harness, Time: ev.Timestamp,
		FromModel: ev.FromModel, ToModel: ev.ToModel,
		FromEffort: ev.RequestedEffort, ToEffort: ev.FinalEffort,
		WantModel: ev.RecommendedModel, Complexity: ev.Complexity,
		Reasons: append([]string(nil), ev.ReasonCodes...),
	}
	if len(ev.SessionID) >= 8 {
		r.Session = ev.SessionID[:8]
	}
	return r
}

// Line is the one human line for the record, used on stderr and in watch.
func (r SpawnRecord) Line() string {
	from, to := shortOrQ(r.FromModel), shortOrQ(r.ToModel)
	var s string
	switch r.State {
	case StateSwitched:
		s = fmt.Sprintf("switched %s → %s", from, to)
		if r.FromModel == r.ToModel {
			s = "switched effort on " + from
		}
		if r.FromEffort != "" && r.FromEffort != "unknown" && r.ToEffort != "" && r.ToEffort != r.FromEffort {
			s += fmt.Sprintf(", effort %s → %s", r.FromEffort, r.ToEffort)
		}
	case StateHeld:
		s = "held on " + from
		if r.WantModel != "" && r.WantModel != "unknown" && r.WantModel != r.FromModel {
			s += " (would be " + ShortModel(r.WantModel) + ")"
		}
		for _, c := range r.Reasons {
			if strings.HasPrefix(c, "HELD_") || c == ReasonCreditHeld {
				s += " [" + c + "]"
			}
		}
	case StatePaused:
		s = "paused, spawn unchanged on " + from
	case StateControl:
		s = "control group, spawn unchanged on " + from
	case StateBlocked:
		s = "blocked on " + from + ", respawn on another model"
	default:
		s = "observed, spawn unchanged on " + from
	}
	if r.Complexity != "" {
		s += " · " + r.Complexity
	}
	return s
}

// switchesModel reports whether a switched spawn ran on another model than the
// requested one. A switch that keeps the model can only have changed the
// effort, so it is an effort change.
func switchesModel(from, to string) bool {
	return !sameModel(from, to)
}

func shortOrQ(id string) string {
	if id == "" || id == "unknown" {
		return "?"
	}
	return ShortModel(id)
}

// LiveState is the file a panel reads: the last spawn plus per-state counts for
// that session. It is rewritten atomically on every spawn.
type LiveState struct {
	Updated string         `json:"updated"`
	Session string         `json:"session,omitempty"`
	Counts  map[string]int `json:"counts"`
	Last    SpawnRecord    `json:"last"`
}

// Active reports whether the last spawn is within ActiveWindow of now.
func (l LiveState) Active(now time.Time) bool {
	t, err := time.Parse(time.RFC3339Nano, l.Updated)
	return err == nil && now.Sub(t) < ActiveWindow && now.Sub(t) > -time.Minute
}

// LivePath is live.json next to the event log, so DOWNSHIFT_EVENT_LOG moves
// both together.
func LivePath() string {
	return filepath.Join(filepath.Dir(defaultEventPath()), "live.json")
}

// WriteLive folds rec into live.json. Counts reset when the session changes.
// It is best effort: the caller ignores the error. A busy lock can cost the
// spawn up to lockTimeout (500 ms), after which the record is skipped.
func WriteLive(rec SpawnRecord) error {
	path := LivePath()
	lock, err := LockFile(path, lockTimeout)
	if err != nil {
		return err
	}
	defer lock.Close()
	cur, _ := ReadLive()
	if cur.Counts == nil || cur.Session != rec.Session {
		cur = LiveState{Counts: map[string]int{}}
	}
	cur.Session, cur.Last = rec.Session, rec
	cur.Counts[rec.State]++
	cur.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// maxLiveBytes caps what ReadLive accepts; a real record is under 1 KB.
const maxLiveBytes = 64 << 10

// ReadLive reads live.json; a missing, oversized or unreadable file is the zero value.
func ReadLive() (LiveState, error) {
	var l LiveState
	f, err := os.Open(LivePath())
	if err != nil {
		return l, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLiveBytes+1))
	if err != nil {
		return l, err
	}
	if len(data) > maxLiveBytes {
		return l, errors.New("live.json too large")
	}
	err = json.Unmarshal(data, &l)
	return l, err
}

// ActivityLine says whether Downshift is working for the session, from its
// event log: the last spawn's state and age, or that nothing was routed yet.
func ActivityLine(events []Event, sessionHash string, now time.Time) string {
	var last *Event
	for _, ev := range sessionEvents(events, sessionHash) {
		if IsDecisionRow(ev) {
			e := ev
			last = &e
		}
	}
	if last == nil {
		return "○ downshift on · no subagent routed yet"
	}
	age := now.Sub(evTime(*last)).Round(time.Second)
	if age < 0 {
		age = 0
	}
	rec := NewSpawnRecord(*last)
	if age < ActiveWindow {
		return fmt.Sprintf("● downshift active · last spawn %s ago: %s", age, rec.Line())
	}
	return fmt.Sprintf("○ downshift idle · last spawn %s ago: %s", age, rec.Line())
}
