// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

// Synthetic prompts only — same structural shapes as the private outcome suite,
// never copied from downshift-labs task.json text.
func TestClassify_OutcomeStructuralRegression(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   Complexity
		tier   Tier
	}{
		{
			name:   "short add exported helper",
			prompt: "add PeakValue(nums []int) (int, bool) returning the largest element or false when the slice is empty",
			want:   Simple,
			tier:   TierSmall,
		},
		{
			name:   "short write spec stays at the safe default",
			prompt: "write TitleCase(word string) string that uppercases only the first rune",
			want:   Medium,
			tier:   TierMid,
		},
		{
			name:   "short write game spec stays mid",
			prompt: "write Fizz(n int) string for multiples of three and five",
			want:   Medium,
			tier:   TierMid,
		},
		{
			name:   "camel to delimiter write stays mid",
			prompt: "write ToSnake(name string) string that turns a camel case identifier into lowercase words separated by underscores",
			want:   Medium,
			tier:   TierMid,
		},
		{
			name: "multi method implement spec",
			prompt: "implement an index: NewIndex() *Index, Insert(term string, rank int) and " +
				"Search(prefix string, limit int) []string ordered by rank then term",
			want: Complex,
			tier: TierFrontier,
		},
		{
			name: "graph shortest path implement",
			prompt: "implement Route(n int, edges [][3]int, from, to int) (cost int, path []int, ok bool) " +
				"for a directed graph with non-negative weights using a binary heap priority queue",
			want: Complex,
			tier: TierFrontier,
		},
		{
			name: "cron style schedule write",
			prompt: "write NextRun(spec string, after time.Time) (time.Time, error) for five-field cron " +
				"schedules with ranges, lists and wildcards in minute hour dom month dow",
			want: Complex,
			tier: TierFrontier,
		},
		{
			name: "mini pattern matcher write",
			prompt: "write Matches(pat, s string) bool for whole-string patterns with literals, dot, " +
				"and postfix operators star plus question applied to the previous atom",
			want: Complex,
			tier: TierFrontier,
		},
		{
			name: "grid solver write",
			prompt: "write Solve(grid [9][9]int) ([9][9]int, bool) that fills a puzzle with backtracking " +
				"and rejects invalid or unsolvable boards",
			want: Complex,
			tier: TierFrontier,
		},
		{
			name: "longer csv record write stays mid",
			prompt: "write SplitRecord(line string) ([]string, error) that parses one delimited record, " +
				"supports quoted fields with embedded delimiters and doubled escape quotes, " +
				"and errors on an unclosed quote",
			want: Medium,
			tier: TierMid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.prompt)
			if got.Complexity != tt.want {
				t.Fatalf("complexity = %s, want %s (scores=%v nosig=%v)", got.Complexity, tt.want, got.Scores, got.NoSignal)
			}
			if got.Complexity.Tier() != tt.tier {
				t.Fatalf("tier = %s, want %s", got.Complexity.Tier(), tt.tier)
			}
		})
	}
}
