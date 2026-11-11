// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

const (
	idA = "0123456789ab"
	idB = "ba9876543210"
)

var t0 = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func row(id string, emitted int64) HonorRow {
	return HonorRow{KeptID: id, EmittedBytes: emitted, At: t0}
}

func sample(id string, n int) hookport.ToolResultSample {
	return hookport.ToolResultSample{ID: id, Bytes: n, Count: 1}
}

func found(samples ...hookport.ToolResultSample) hookport.ReadToolResultsFunc {
	return func([]string) hookport.ToolResults {
		return hookport.ToolResults{Readable: true, Samples: samples}
	}
}

func TestClassifyHonor(t *testing.T) {
	cases := []struct {
		name string
		rows []HonorRow
		read hookport.ReadToolResultsFunc
		want HonorCounts
	}{
		{"honored, transcript drops the trailing newline", []HonorRow{row(idA, 1000)},
			found(sample(idA, 999)), HonorCounts{Honored: 1}},
		{"honored, same length", []HonorRow{row(idA, 1000)},
			found(sample(idA, 1000)), HonorCounts{Honored: 1}},
		{"length off by two is a mismatch", []HonorRow{row(idA, 1000)},
			found(sample(idA, 998)), HonorCounts{Mismatch: 1}},
		{"longer than emitted is a mismatch", []HonorRow{row(idA, 1000)},
			found(sample(idA, 1001)), HonorCounts{Mismatch: 1}},
		{"pre-run marker with the same id and another length does not count", []HonorRow{row(idA, 1000)},
			found(sample(idA, 4000)), HonorCounts{Mismatch: 1}},
		{"one matching length among several candidates is honored", []HonorRow{row(idA, 1000)},
			found(sample(idA, 4000), sample(idA, 999)), HonorCounts{Honored: 1}},
		{"id absent from the transcripts is not found", []HonorRow{row(idA, 1000)},
			found(sample(idB, 1000)), HonorCounts{NotFound: 1}},
		{"no kept id is unverified", []HonorRow{row("", 1000)},
			found(), HonorCounts{Unverified: 1}},
		{"kept id with zero emitted bytes is unverified", []HonorRow{row(idA, 0)},
			found(sample(idA, 0)), HonorCounts{Unverified: 1}},
		{"harness without a transcript reader: unverified", []HonorRow{row(idA, 1000), row("", 5)},
			nil, HonorCounts{Unverified: 2}},
		{"mixed", []HonorRow{row(idA, 10), row(idB, 20), row("", 30)},
			found(sample(idA, 9)), HonorCounts{Honored: 1, NotFound: 1, Unverified: 1}},
		{"no rows", nil, found(), HonorCounts{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyHonor(c.rows, c.read); got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestClassifyHonorUnreadableIsUnverified(t *testing.T) {
	read := func([]string) hookport.ToolResults { return hookport.ToolResults{} }
	if got := ClassifyHonor([]HonorRow{row(idA, 10)}, read); got != (HonorCounts{Unverified: 1}) {
		t.Fatalf("got %+v", got)
	}
}

func TestClassifyHonorConsumesEachResultOnce(t *testing.T) {
	// Two rows kept the same output. One transcript result may honor only one.
	one := ClassifyHonor([]HonorRow{row(idA, 100), row(idA, 100)}, found(sample(idA, 99)))
	if one != (HonorCounts{Honored: 1, Mismatch: 1}) {
		t.Fatalf("one result, two rows: %+v", one)
	}
	two := hookport.ToolResultSample{ID: idA, Bytes: 99, Count: 2}
	if got := ClassifyHonor([]HonorRow{row(idA, 100), row(idA, 100)}, found(two)); got != (HonorCounts{Honored: 2}) {
		t.Fatalf("two results, two rows: %+v", got)
	}
}

func TestClassifyHonorSlackCollision(t *testing.T) {
	// Rows of 100 and 101 bytes, results of 99 and 100: the usual one-byte
	// drop pairs 100 with 99 and 101 with 100.
	rows := []HonorRow{row(idA, 100), row(idA, 101)}
	got := ClassifyHonor(rows, found(sample(idA, 99), sample(idA, 100)))
	if got != (HonorCounts{Honored: 2}) {
		t.Fatalf("got %+v", got)
	}
	// One result of 100 bytes fits both rows by slack, but honors only the first.
	got = ClassifyHonor(rows, found(sample(idA, 100)))
	if got != (HonorCounts{Honored: 1, Mismatch: 1}) {
		t.Fatalf("got %+v", got)
	}
}

func TestClassifyHonorCoverage(t *testing.T) {
	since := t0.Add(time.Hour)
	read := func(truncated bool, samples ...hookport.ToolResultSample) hookport.ReadToolResultsFunc {
		return func([]string) hookport.ToolResults {
			return hookport.ToolResults{Readable: true, Samples: samples, Since: since, Truncated: truncated}
		}
	}
	older := row(idA, 100) // logged before the coverage starts
	newer := HonorRow{KeptID: idA, EmittedBytes: 100, At: since.Add(time.Minute)}
	if got := ClassifyHonor([]HonorRow{older}, read(false, sample(idA, 99))); got != (HonorCounts{Unverified: 1}) {
		t.Errorf("older than the coverage: %+v", got)
	}
	if got := ClassifyHonor([]HonorRow{newer}, read(false, sample(idA, 99))); got != (HonorCounts{Honored: 1}) {
		t.Errorf("inside the coverage: %+v", got)
	}
	if got := ClassifyHonor([]HonorRow{newer}, read(false)); got != (HonorCounts{NotFound: 1}) {
		t.Errorf("complete read, id absent: %+v", got)
	}
	if got := ClassifyHonor([]HonorRow{newer}, read(true)); got != (HonorCounts{Unverified: 1}) {
		t.Errorf("truncated read, id absent must be unverified: %+v", got)
	}
	if got := ClassifyHonor([]HonorRow{{KeptID: idA, EmittedBytes: 100}}, read(false, sample(idA, 99))); got != (HonorCounts{Unverified: 1}) {
		t.Errorf("a row with no time cannot be placed in the coverage: %+v", got)
	}
}

func TestClassifyHonorAsksOnlyForKeptIDs(t *testing.T) {
	var asked []string
	read := func(ids []string) hookport.ToolResults { asked = ids; return hookport.ToolResults{Readable: true} }
	ClassifyHonor([]HonorRow{row(idA, 1), row("", 2), row(idA, 3), row(idB, 0)}, read)
	if len(asked) != 1 || asked[0] != idA {
		t.Fatalf("asked %v, want one distinct id", asked)
	}
	asked = nil
	ClassifyHonor([]HonorRow{row("", 2)}, read)
	if asked != nil {
		t.Fatalf("read must not be called without ids, got %v", asked)
	}
}
