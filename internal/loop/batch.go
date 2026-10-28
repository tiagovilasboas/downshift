// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Limits on a batch file, so a wrong path or a runaway generator cannot make
// the runner read or queue without bound.
const (
	maxBatchBytes = 1 << 20
	maxBatchItems = 500
)

// Item is one entry of a batch file: a task, the sensor that verifies it and
// optionally the model name recorded in the cascade record when it fails (the
// loop does not switch models). Sensor and Model fall back to the base
// Config when empty.
type Item struct {
	Task   string `json:"task"`
	Sensor string `json:"sensor,omitempty"`
	Model  string `json:"model,omitempty"`
}

// ParseBatch reads a JSON array of items. Unknown fields, an empty list and an
// item without a task are errors, so a typo in a key never runs a blank task.
func ParseBatch(r io.Reader) ([]Item, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBatchBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read batch: %w", err)
	}
	if len(data) > maxBatchBytes {
		return nil, fmt.Errorf("batch file is larger than %d bytes", maxBatchBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var items []Item
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("parse batch: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("parse batch: trailing data after the array")
	}
	if len(items) == 0 {
		return nil, errors.New("batch has no tasks")
	}
	if len(items) > maxBatchItems {
		return nil, fmt.Errorf("batch has %d tasks, the limit is %d", len(items), maxBatchItems)
	}
	for i, it := range items {
		if strings.TrimSpace(it.Task) == "" {
			return nil, fmt.Errorf("batch item %d has no task", i+1)
		}
	}
	return items, nil
}

// ValidateBatch refuses a batch where a task would run with no sensor. A run
// with nothing to verify it would pass and be recorded as accepted, so a batch
// must say how each task is checked, either per item or through the base sensor.
func ValidateBatch(items []Item, base Config) error {
	for i, it := range items {
		if it.Sensor == "" && base.SensorCmd == "" {
			return fmt.Errorf("batch item %d has no sensor and no --sensor default", i+1)
		}
	}
	return nil
}

// RunFunc runs one loop. The default is the real Runner; tests inject a fake.
type RunFunc func(ctx context.Context, cfg Config) (Result, error)

// BatchConfig is one batch run. Base carries the settings every item shares.
type BatchConfig struct {
	Items           []Item
	Base            Config
	ContinueOnError bool
	Run             RunFunc
}

// ItemResult is the outcome of one item. Index is 1-based.
type ItemResult struct {
	Index   int    `json:"index"`
	Task    string `json:"task"`
	Skipped bool   `json:"skipped,omitempty"`
	Result
}

// BatchResult sums a batch. Recovered counts items that passed with the
// cascade active, that is, after a recorded failure.
type BatchResult struct {
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Skipped   int           `json:"skipped"`
	Recovered int           `json:"recovered"`
	Duration  time.Duration `json:"duration"`
	Items     []ItemResult  `json:"items"`
}

// OK reports whether every item ran and passed.
func (b BatchResult) OK() bool { return b.Failed == 0 && b.Skipped == 0 }

// RunBatch runs the items in order, one loop each. It stops at the first
// failure unless ContinueOnError is set; the items left over are reported as
// skipped. A cancelled context stops the batch the same way.
func RunBatch(ctx context.Context, cfg BatchConfig) BatchResult {
	run := cfg.Run
	if run == nil {
		run = func(ctx context.Context, c Config) (Result, error) { return NewRunner(c).Run(ctx) }
	}
	start := time.Now()
	out := BatchResult{Total: len(cfg.Items)}
	stopped := false
	for i, it := range cfg.Items {
		ir := ItemResult{Index: i + 1, Task: it.Task}
		if stopped || ctx.Err() != nil {
			stopped = true
			ir.Skipped = true
			out.Skipped++
			out.Items = append(out.Items, ir)
			continue
		}
		res, err := run(ctx, itemConfig(cfg.Base, it))
		if err != nil && res.Message == "" {
			res.Message = err.Error()
		}
		ir.Result = res
		if res.Success {
			out.Passed++
			if res.CascadeActive {
				out.Recovered++
			}
		} else {
			out.Failed++
			stopped = !cfg.ContinueOnError
		}
		out.Items = append(out.Items, ir)
	}
	out.Duration = time.Since(start)
	return out
}

// itemConfig overlays an item on the shared settings.
func itemConfig(base Config, it Item) Config {
	c := base
	c.Task = it.Task
	if it.Sensor != "" {
		c.SensorCmd = it.Sensor
	}
	if it.Model != "" {
		c.Model = it.Model
	}
	return c
}
