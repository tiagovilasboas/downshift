// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package antigravitybridge reads the native Antigravity CLI outside hooks.
// It preserves parent picker and grouped quota evidence without inventing an
// association with invoke_subagent aliases or routing quota coverage.
package antigravitybridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const maxOutput = 1 << 20

type Model struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Bucket struct {
	ID                string    `json:"id"`
	Window            string    `json:"window"`
	RemainingFraction *float64  `json:"remaining_fraction"`
	ResetTime         time.Time `json:"reset_time"`
}

type Group struct {
	Name    string   `json:"name"`
	Buckets []Bucket `json:"buckets"`
}

// Observation is native evidence, not a routing Snapshot. MappingResolved is
// always false: neither verified native payload supplies structural membership
// between groups, parent model IDs and invoke_subagent model tiers.
type Observation struct {
	Source          string    `json:"source"`
	ObservedAt      time.Time `json:"observed_at"`
	ModelNamespace  string    `json:"model_namespace"`
	Models          []Model   `json:"models"`
	QuotaGroups     []Group   `json:"quota_groups"`
	MappingResolved bool      `json:"mapping_resolved"`
}

type envelope struct {
	Status         string             `json:"status"`
	ConversationID string             `json:"conversation_id"`
	Turns          int                `json:"num_turns"`
	Usage          map[string]float64 `json:"usage"`
	Command        struct {
		Name string          `json:"name"`
		Data json.RawMessage `json:"data"`
	} `json:"command"`
}

func decode(data []byte, command string, target any) error {
	if len(data) > maxOutput {
		return errors.New("Antigravity native output exceeds limit")
	}
	var env envelope
	if json.Unmarshal(data, &env) != nil || env.Status != "SUCCESS" || env.Command.Name != command || len(env.Command.Data) == 0 || env.ConversationID != "" || env.Turns != 0 {
		return errors.New("expected a successful native Antigravity read-only command")
	}
	for _, n := range env.Usage {
		if n != 0 {
			return errors.New("Antigravity read-only command reported token consumption")
		}
	}
	if err := json.Unmarshal(env.Command.Data, target); err != nil {
		return errors.New("invalid Antigravity command data")
	}
	return nil
}

var modelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/+-]*$`)

// Parse consumes only structured native fields. It deliberately drops response
// text and human-readable quota descriptions; neither defines pool membership.
func Parse(models, usage []byte, observed time.Time) (Observation, error) {
	o := Observation{Source: "antigravity-cli", ObservedAt: observed, ModelNamespace: "parent-picker"}
	if observed.IsZero() {
		return o, errors.New("Antigravity observation requires timestamp")
	}
	var m struct {
		Models []Model `json:"models"`
	}
	if err := decode(models, "models", &m); err != nil {
		return o, err
	}
	if len(m.Models) == 0 {
		return o, errors.New("Antigravity parent picker is empty")
	}
	seenModels := map[string]bool{}
	for _, model := range m.Models {
		if !modelID.MatchString(model.ID) || strings.TrimSpace(model.Label) == "" || seenModels[model.ID] {
			return o, errors.New("invalid or duplicate Antigravity parent model")
		}
		seenModels[model.ID] = true
	}
	var u struct {
		Groups []Group `json:"groups"`
	}
	if err := decode(usage, "usage", &u); err != nil {
		return o, err
	}
	if len(u.Groups) == 0 {
		return o, errors.New("Antigravity quota groups are empty")
	}
	seenGroups, seenBuckets := map[string]bool{}, map[string]bool{}
	for _, group := range u.Groups {
		if strings.TrimSpace(group.Name) == "" || seenGroups[group.Name] || len(group.Buckets) == 0 {
			return o, errors.New("invalid or duplicate Antigravity quota group")
		}
		seenGroups[group.Name] = true
		for _, bucket := range group.Buckets {
			if bucket.ID == "" || seenBuckets[bucket.ID] || bucket.Window == "" || bucket.RemainingFraction == nil || bucket.ResetTime.IsZero() {
				return o, errors.New("incomplete or duplicate Antigravity quota bucket")
			}
			fraction := *bucket.RemainingFraction
			if math.IsNaN(fraction) || math.IsInf(fraction, 0) || fraction < 0 || fraction > 1 {
				return o, errors.New("invalid Antigravity remaining_fraction")
			}
			seenBuckets[bucket.ID] = true
		}
	}
	o.Models, o.QuotaGroups = m.Models, u.Groups
	return o, nil
}

// Collector invokes the native CLI with closed stdin and bounded output. A
// deadline applies to the entire collection. Authentication remains inside the
// CLI; diagnostics are discarded so auth URLs and credentials cannot leak.
type Collector struct {
	Bin     string
	Timeout time.Duration
}

func (c Collector) Collect(ctx context.Context) (Observation, error) {
	bin := c.Bin
	if bin == "" {
		bin = "agy"
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	help, err := run(ctx, bin, "--help")
	if err != nil {
		return Observation{}, err
	}
	if !bytes.Contains(help, []byte("Usage of agy:")) || !bytes.Contains(help, []byte("List available models")) || !bytes.Contains(help, []byte("--output-format")) || !bytes.Contains(help, []byte("--print")) {
		return Observation{}, errors.New("executable is not the supported native Antigravity CLI")
	}
	models, err := run(ctx, bin, "--output-format", "json", "models")
	if err != nil {
		return Observation{}, err
	}
	usage, err := run(ctx, bin, "-p", "/usage", "--output-format", "json")
	if err != nil {
		return Observation{}, err
	}
	return Parse(models, usage, time.Now().UTC())
}

type bounded struct{ bytes.Buffer }

func (b *bounded) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxOutput {
		return 0, errors.New("native output exceeds limit")
	}
	return b.Buffer.Write(p)
}

func run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = time.Second
	var out bounded
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Antigravity native collection: %w", ctx.Err())
		}
		return nil, errors.New("Antigravity native command failed; verify CLI installation and existing authentication")
	}
	return out.Bytes(), nil
}
