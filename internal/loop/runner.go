// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/cascade"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Runner coordinates the autonomous loop execution.
type Runner struct {
	cfg Config
}

// NewRunner creates a new loop runner with sane defaults.
func NewRunner(cfg Config) *Runner {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	if cfg.Harness == "" {
		cfg.Harness = "claude-code"
	}
	return &Runner{cfg: cfg}
}

// Run executes the autonomous engineering loop:
// Plan -> Implement -> Sensor Check -> Auto-Upgrade Cascade (if failed) -> Accept & Commit.
func (r *Runner) Run(ctx context.Context) (Result, error) {
	start := time.Now()
	res := Result{
		FinalStep: StepPlan,
	}

	loopCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	for iter := 1; iter <= r.cfg.MaxIterations; iter++ {
		res.Iterations = iter

		select {
		case <-loopCtx.Done():
			res.FinalStep = StepFailed
			res.Message = "loop aborted: timeout or context cancelled"
			res.Duration = time.Since(start)
			return res, loopCtx.Err()
		default:
		}

		// 1. Check if Cascade Auto-Upgrade is active for this task
		_, hasFailure := cascade.HasRecentFailure(r.cfg.SessionID, r.cfg.Task, 15*time.Minute)
		if hasFailure {
			res.CascadeActive = true
		}

		// 2. Run sensor check if command is supplied
		res.FinalStep = StepSensor
		if r.cfg.SensorCmd != "" {
			exitCode, err := runSensor(loopCtx, r.cfg.SensorCmd)
			if err != nil || exitCode != 0 {
				// Sensor failed: record dead-end to trigger Cascade Auto-Upgrade on next turn
				res.SensorPassed = false
				res.FinalStep = StepCascade
				failedModel := r.cfg.Model
				if failedModel == "" {
					failedModel = r.cfg.Harness + "-small"
				}
				cascade.RecordFailure(
					r.cfg.SessionID,
					r.cfg.Task,
					failedModel,
					fmt.Sprintf("sensor %q failed with exit code %d (err: %v)", r.cfg.SensorCmd, exitCode, err),
				)

				if iter == r.cfg.MaxIterations {
					res.FinalStep = StepFailed
					res.Message = fmt.Sprintf("tripwire stop: sensor failed after %d iterations", iter)
					res.Duration = time.Since(start)
					return res, nil
				}
				continue
			}
		}

		// Sensor passed (or no sensor command): mark accepted
		res.SensorPassed = true
		res.FinalStep = StepAccept
		if r.cfg.SessionID != "" {
			_ = telemetry.RecordAcceptance(r.cfg.SessionID, telemetry.AcceptanceUsed, r.cfg.Harness)
			cascade.ClearTask(r.cfg.SessionID, r.cfg.Task)
			res.Accepted = true
		}

		res.Success = true
		res.FinalStep = StepComplete
		res.Message = "loop completed successfully with verified sensor"
		res.Duration = time.Since(start)
		return res, nil
	}

	res.FinalStep = StepFailed
	res.Duration = time.Since(start)
	return res, nil
}

func runSensor(ctx context.Context, cmdStr string) (int, error) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return 0, nil
	}

	if execwrap.FamilyOf(parts) != execwrap.FamilyNone {
		var stdoutBuf, stderrBuf strings.Builder
		res := execwrap.Run(parts, execwrap.Options{
			Stdout:  &stdoutBuf,
			Stderr:  &stderrBuf,
			Compact: true,
		})
		return res.ExitCode, nil
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
