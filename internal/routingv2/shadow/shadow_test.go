// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shadow

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/classifier"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/domain"
)

type predictorFunc func(Input) (domain.TierProbabilities, error)

func (f predictorFunc) Predict(i Input) (domain.TierProbabilities, error) { return f(i) }

const testModelID = "softmax-sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestEvaluateKeepsRawErrorVisibleBehindSafetyFloor(t *testing.T) {
	p := predictorFunc(func(i Input) (domain.TierProbabilities, error) {
		if i.Text != "private task" {
			t.Fatal("semantic backends must receive transient text")
		}
		return domain.TierProbabilities{Small: .95, Mid: .04, Frontier: .01}, nil
	})
	o := Evaluate(p, testModelID, Input{Text: "private task", Features: domain.FeatureVector{Security: .9}})
	if !o.Valid() || o.RawTier != core.TierSmall || o.PolicyTier != core.TierFrontier || o.SafetyFloor != core.TierFrontier {
		t.Fatalf("unsafe raw prediction must survive policy correction: %+v", o)
	}
}

func TestEvaluateRejectsBadBackendsWithoutPersistingErrors(t *testing.T) {
	tests := []struct {
		name string
		p    predictorFunc
		code string
	}{
		{"error", func(Input) (domain.TierProbabilities, error) {
			return domain.TierProbabilities{}, errors.New("secret task")
		}, "predict_error"},
		{"panic", func(Input) (domain.TierProbabilities, error) { panic("secret task") }, "predict_panic"},
		{"nan", func(Input) (domain.TierProbabilities, error) { return domain.TierProbabilities{Small: math.NaN()}, nil }, "invalid_probabilities"},
		{"negative", func(Input) (domain.TierProbabilities, error) {
			return domain.TierProbabilities{Small: -.2, Mid: 1.2}, nil
		}, "invalid_probabilities"},
		{"sum", func(Input) (domain.TierProbabilities, error) {
			return domain.TierProbabilities{Small: .2, Mid: .2, Frontier: .2}, nil
		}, "invalid_probabilities"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := Evaluate(tt.p, testModelID, Input{})
			if o.ErrorCode != tt.code || o.Probabilities != nil || !o.Valid() {
				t.Fatalf("bad backend observation: %+v", o)
			}
		})
	}
}

func TestFromEnvOptInAndArtifactIdentity(t *testing.T) {
	t.Setenv("DOWNSHIFT_SHADOW_WEIGHTS", "")
	if FromEnv(Input{}) != nil {
		t.Fatal("disabled shadow must produce no observation")
	}
	path := filepath.Join(t.TempDir(), "candidate.json")
	if err := classifier.SaveWeights(classifier.DefaultWeights(), path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_SHADOW_WEIGHTS", path)
	first := FromEnv(Input{})
	second := FromEnv(Input{})
	if first == nil || !first.Valid() || first.ModelID != second.ModelID || strings.Contains(first.ModelID, path) {
		t.Fatalf("artifact identity not reproducible: %+v / %+v", first, second)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if first.ModelID == FromEnv(Input{}).ModelID {
		t.Fatal("changed artifact must have different identity")
	}
	for _, value := range []string{"not json", strings.Repeat("x", (64<<10)+1), `{"small":{"bias":1000001}}`} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		o := FromEnv(Input{})
		if o.ErrorCode != "load_error" || o.ModelID != "" {
			t.Fatalf("invalid artifact accepted: %+v", o)
		}
	}
	t.Setenv("DOWNSHIFT_SHADOW_WEIGHTS", t.TempDir())
	if FromEnv(Input{}).ErrorCode != "load_error" {
		t.Fatal("directories must be rejected")
	}
	t.Setenv("DOWNSHIFT_SHADOW_WEIGHTS", path+"-missing")
	if FromEnv(Input{}).ErrorCode != "load_error" {
		t.Fatal("missing artifact must be advisory failure")
	}
}
