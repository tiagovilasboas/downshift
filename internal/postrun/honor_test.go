// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

const (
	idA = "0123456789ab"
	idB = "ba9876543210"
)

func reader(ok bool, samples ...hookport.ToolResultSample) hookport.ReadToolResultsFunc {
	return func(ids []string) ([]hookport.ToolResultSample, bool) { return samples, ok }
}

func sample(id string, n int) hookport.ToolResultSample {
	return hookport.ToolResultSample{ID: id, Bytes: n}
}

func TestClassifyHonor(t *testing.T) {
	cases := []struct {
		name string
		rows []HonorRow
		read hookport.ReadToolResultsFunc
		want HonorCounts
	}{
		{"honored, transcript drops the trailing newline", []HonorRow{{idA, 1000}},
			reader(true, sample(idA, 999)), HonorCounts{Honored: 1}},
		{"honored, same length", []HonorRow{{idA, 1000}},
			reader(true, sample(idA, 1000)), HonorCounts{Honored: 1}},
		{"length off by two is a mismatch", []HonorRow{{idA, 1000}},
			reader(true, sample(idA, 998)), HonorCounts{Mismatch: 1}},
		{"longer than emitted is a mismatch", []HonorRow{{idA, 1000}},
			reader(true, sample(idA, 1001)), HonorCounts{Mismatch: 1}},
		{"pre-run marker with the same id and another length does not count", []HonorRow{{idA, 1000}},
			reader(true, sample(idA, 4000)), HonorCounts{Mismatch: 1}},
		{"one matching length among several candidates is honored", []HonorRow{{idA, 1000}},
			reader(true, sample(idA, 4000), sample(idA, 999)), HonorCounts{Honored: 1}},
		{"id absent from the transcripts is not found", []HonorRow{{idA, 1000}},
			reader(true, sample(idB, 1000)), HonorCounts{NotFound: 1}},
		{"no kept id is unverified", []HonorRow{{"", 1000}},
			reader(true), HonorCounts{Unverified: 1}},
		{"transcripts unreadable: unverified", []HonorRow{{idA, 1000}},
			reader(false), HonorCounts{Unverified: 1}},
		{"harness without a transcript reader: unverified", []HonorRow{{idA, 1000}, {"", 5}},
			nil, HonorCounts{Unverified: 2}},
		{"mixed", []HonorRow{{idA, 10}, {idB, 20}, {"", 30}},
			reader(true, sample(idA, 9)), HonorCounts{Honored: 1, NotFound: 1, Unverified: 1}},
		{"no rows", nil, reader(true), HonorCounts{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyHonor(c.rows, c.read); got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestClassifyHonorAsksOnlyForKeptIDs(t *testing.T) {
	var asked []string
	read := func(ids []string) ([]hookport.ToolResultSample, bool) { asked = ids; return nil, true }
	ClassifyHonor([]HonorRow{{idA, 1}, {"", 2}, {idA, 3}}, read)
	if len(asked) != 1 || asked[0] != idA {
		t.Fatalf("asked %v, want one distinct id", asked)
	}
	asked = nil
	ClassifyHonor([]HonorRow{{"", 2}}, read)
	if asked != nil {
		t.Fatalf("read must not be called without ids, got %v", asked)
	}
}
