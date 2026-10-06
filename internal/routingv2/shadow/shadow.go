// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

// Package shadow compares candidate classifiers without changing production.
// Part of harness-downshift by Tiago de Carvalho Vilas Boas.
// https://github.com/tiagovilasboas/harness-downshift
package shadow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/classifier"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/domain"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/policy"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/safety"
)

// Input stays in memory. Text lets a future semantic backend consume the task
// without changing domain.Classifier, whose input is only thirteen signals.
type Input struct {
	Text     string
	Features domain.FeatureVector
}

// Predictor is an experiment boundary, not a production routing interface.
// Implementations must be local, bounded and must not persist Input.Text.
type Predictor interface {
	Predict(Input) (domain.TierProbabilities, error)
}

// Observation contains only numeric outputs and allowlisted artifact metadata.
// PolicyTier is a hypothetical recommendation, never an executed model tier.
type Observation struct {
	ModelID       string                    `json:"model_id"`
	Probabilities *domain.TierProbabilities `json:"probabilities,omitempty"`
	RawTier       core.Tier                 `json:"raw_tier"`
	PolicyTier    core.Tier                 `json:"policy_tier"`
	SafetyFloor   core.Tier                 `json:"safety_floor"`
	Margin        float64                   `json:"margin"`
	ErrorCode     string                    `json:"error_code,omitempty"`
}

// Evaluate never returns a decision to a router. Raw and policy-adjusted tiers
// are both retained so safety floors cannot hide a weak classifier.
func Evaluate(p Predictor, modelID string, input Input) (o Observation) {
	o.ModelID = modelID
	defer func() {
		if recover() != nil {
			o = Observation{ModelID: modelID, ErrorCode: "predict_panic"}
		}
	}()
	probs, err := p.Predict(input)
	if err != nil {
		o.ErrorCode = "predict_error" // Never persist backend errors or task text.
		return o
	}
	if !validProbabilities(probs) {
		o.ErrorCode = "invalid_probabilities"
		return o
	}
	floor := safety.NewEvaluator().Evaluate(input.Features)
	o.Probabilities = &probs
	o.RawTier = probs.MaxTier()
	o.PolicyTier = policy.Decide(probs, floor)
	o.SafetyFloor = floor.MinTier
	o.Margin = probs.Confidence()
	return o
}

func validProbabilities(p domain.TierProbabilities) bool {
	for _, v := range []float64{p.Small, p.Mid, p.Frontier} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return false
		}
	}
	return math.Abs(p.Small+p.Mid+p.Frontier-1) <= 1e-6
}

// Valid checks replayed observations before a report includes them.
func (o Observation) Valid() bool {
	const prefix = "softmax-sha256:"
	if !strings.HasPrefix(o.ModelID, prefix) {
		return false
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(o.ModelID, prefix))
	if err != nil || len(digest) != sha256.Size {
		return false
	}
	if o.ErrorCode != "" {
		return o.Probabilities == nil && (o.ErrorCode == "predict_error" || o.ErrorCode == "predict_panic" || o.ErrorCode == "invalid_probabilities")
	}
	if o.Probabilities == nil || !validProbabilities(*o.Probabilities) ||
		o.SafetyFloor < core.TierSmall || o.SafetyFloor > core.TierFrontier ||
		math.IsNaN(o.Margin) || math.IsInf(o.Margin, 0) {
		return false
	}
	floor := domain.SafetyConstraint{MinTier: o.SafetyFloor}
	return o.RawTier == o.Probabilities.MaxTier() &&
		o.PolicyTier == policy.Decide(*o.Probabilities, floor) &&
		math.Abs(o.Margin-o.Probabilities.Confidence()) <= 1e-6
}

type featurePredictor struct{ classifier domain.Classifier }

func (p featurePredictor) Predict(input Input) (domain.TierProbabilities, error) {
	return p.classifier.Classify(input.Features)
}

// FromEnv is opt-in and never reads production weights or changes activation.
// Candidate artifacts are small regular JSON files; FIFOs/devices are rejected.
func FromEnv(input Input) *Observation {
	path := strings.TrimSpace(os.Getenv("DOWNSHIFT_SHADOW_WEIGHTS"))
	if path == "" {
		return nil
	}
	p, id, err := loadCandidate(path)
	if err != nil {
		return &Observation{ErrorCode: "load_error"}
	}
	o := Evaluate(p, id, input)
	return &o
}

func loadCandidate(path string) (Predictor, string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("candidate must be a regular file")
	}
	const maxBytes = 64 << 10
	if info.Size() > maxBytes {
		return nil, "", fmt.Errorf("candidate too large")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil || len(data) > maxBytes {
		return nil, "", fmt.Errorf("reading candidate")
	}
	var w classifier.Weights
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, "", err
	}
	if err := w.Validate(); err != nil {
		return nil, "", err
	}
	// Hash the same bytes used for inference. Do not trust free-text version
	// fields or store a local path, which could disclose task/customer metadata.
	digest := sha256.Sum256(data)
	return featurePredictor{classifier.NewSoftmaxClassifier(&w)}, "softmax-sha256:" + hex.EncodeToString(digest[:]), nil
}
