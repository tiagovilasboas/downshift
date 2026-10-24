// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package harness2export

// SmokeDataset is the public CLI fixture. It is not a promotion gate.
const SmokeDataset = "benchmark/routing-tasks.public.json"

// LabReceipt is the versioned result of a local lab run. Routing code stays
// unchanged. Commit and push this file so the remote checkout carries the same result.
type LabReceipt struct {
	SchemaVersion         int      `json:"schema_version"`
	Kind                  string   `json:"kind"`
	MutationTargets       []string `json:"mutation_targets"`
	ObservationTargets    []string `json:"observation_targets"`
	ClassifierOnlyTargets []string `json:"classifier_only_targets"`
	SmokeDataset          string   `json:"smoke_dataset"`
	SmokePromotionGate    bool     `json:"smoke_promotion_gate"`
	Judge                 string   `json:"judge"`
	JudgePassed           bool     `json:"judge_passed"`
	Effect                string   `json:"effect"`
}

// BuildReceipt returns the deterministic receipt for a matrix that passes Judge.
func BuildReceipt(rows []Row) (LabReceipt, error) {
	if err := Judge(rows); err != nil {
		return LabReceipt{}, err
	}
	rec := LabReceipt{
		SchemaVersion:      1,
		Kind:               "local_lab_receipt",
		SmokeDataset:       SmokeDataset,
		SmokePromotionGate: false,
		Judge:              "matrix_role_invariant",
		JudgePassed:        true,
		Effect:             "Receipt is committed in this checkout. Push it so the remote project carries the same lab result. Routing policy is unchanged.",
	}
	for _, row := range rows {
		switch row.Role {
		case RoleMutate:
			if row.RecursiveHarness2Eligible {
				rec.MutationTargets = append(rec.MutationTargets, row.ID)
			}
		case RoleObserve:
			rec.ObservationTargets = append(rec.ObservationTargets, row.ID)
		case RoleClassifierOnly:
			rec.ClassifierOnlyTargets = append(rec.ClassifierOnlyTargets, row.ID)
		}
	}
	return rec, nil
}
