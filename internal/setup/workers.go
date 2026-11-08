// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// workersTarget labels the plan in the output. The file belongs to the user,
// not to a harness.
var workersTarget = Target{Name: "workers"}

// ValidWorkersMode reports whether v is one of the three values of
// orchestration.workers.
func ValidWorkersMode(v string) bool {
	return v == policy.WorkersOff || v == policy.WorkersRead || v == policy.WorkersWrite
}

// planWorkers plans the user policy file with orchestration.workers set to
// o.Workers, keeping every other key. It never writes. ok is false when no
// answer was given (no flag, no wizard choice), so --yes alone writes nothing.
func planWorkers(o Options) (plan Plan, ok bool) {
	if o.Workers == "" {
		return Plan{}, false
	}
	path := filepath.Join(o.StatePath(), policy.FileName)
	plan = Plan{Target: workersTarget, Path: path}
	if st, err := os.Lstat(path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		plan.Err = errors.New("is a link; setup leaves linked files alone (edit the real file by hand, or remove the link)")
		return plan, true
	}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		plan.Existed, plan.Before = true, raw
	case errors.Is(err, os.ErrNotExist):
	default:
		plan.Err = err
		return plan, true
	}
	doc, err := configfile.ParseObject(raw)
	if err != nil {
		plan.Err = fmt.Errorf("not valid JSON, left untouched: %w", err)
		return plan, true
	}
	if !plan.Existed {
		doc.Set("version", 1)
	}
	orch := configfile.NewObject()
	if v, found := doc.Get("orchestration"); found {
		cur, isObj := configfile.AsObject(v)
		if !isObj {
			plan.Err = errors.New("orchestration is not an object, left untouched")
			return plan, true
		}
		orch = cur
	}
	orch.Set("workers", o.Workers)
	doc.Set("orchestration", orch)
	if plan.After, err = configfile.RenderObject(doc); err != nil {
		plan.Err = err
	}
	return plan, true
}
