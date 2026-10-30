// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// appliedPlan records what one plan has done so far, so it can be undone.
type appliedPlan struct {
	plan    agentPlan
	backup  string // backup file created for this plan, "" when none
	written bool   // the agent file itself was replaced
}

// commitPlans writes every plan or none. Every plan has already passed
// planAgent, so a failure here is an I/O error. On the first one it undoes
// whatever this run changed (agent files restored or removed, backups removed)
// and says whether the undo was complete.
func commitPlans(plans []agentPlan, stamp string) error {
	var done []appliedPlan
	for _, p := range plans {
		a := appliedPlan{plan: p}
		err := writePlan(&a, stamp)
		done = append(done, a)
		if err != nil {
			if undoErr := rollbackPlans(done); undoErr != nil {
				return fmt.Errorf("%w; the undo is incomplete: %v", err, undoErr)
			}
			return fmt.Errorf("%w; everything this run changed was restored", err)
		}
	}
	return nil
}

func writePlan(a *appliedPlan, stamp string) error {
	p := a.plan
	if p.existed {
		bak := p.path + setupBackupInfix + stamp
		if err := createExclusive(bak, p.before, 0o600); err != nil {
			return fmt.Errorf("%s: backup failed: %w", p.name, err)
		}
		a.backup = bak
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return fmt.Errorf("%s: cannot create directory: %w", p.name, err)
	}
	if err := atomicWriteFile(p.path, []byte(p.after), p.mode); err != nil {
		return fmt.Errorf("%s: write failed: %w", p.name, err)
	}
	a.written = true
	return nil
}

// rollbackPlans undoes done in reverse order and returns every error it hit.
func rollbackPlans(done []appliedPlan) error {
	var errs []error
	for i := len(done) - 1; i >= 0; i-- {
		a := done[i]
		if a.written {
			if a.plan.existed {
				errs = append(errs, atomicWriteFile(a.plan.path, a.plan.before, a.plan.mode))
			} else {
				errs = append(errs, os.Remove(a.plan.path))
			}
		}
		if a.backup != "" {
			errs = append(errs, os.Remove(a.backup))
		}
	}
	return errors.Join(errs...)
}

// createExclusive creates path and writes data to it. It fails when path exists,
// so a backup never follows a link or overwrites an earlier backup.
func createExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}
