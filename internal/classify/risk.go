// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

type riskSignal struct {
	category string
	re       *regexp.Regexp
	literals []string // nil: no prefilter, always run re
}

// Compiled on first use, not at package init (see signals).
var riskSignals = sync.OnceValue(func() []riskSignal {
	out := make([]riskSignal, 0, len(RawRiskSignals))
	for _, r := range RawRiskSignals {
		out = append(out, riskSignal{
			category: r.Category,
			re:       regexp.MustCompile(`(?i)` + r.Pattern),
			literals: riskLiterals[r.Category],
		})
	}
	return out
})

// mayMatch reports whether folded contains a literal the regex needs. Without
// the prefilter it reports true, so the regex decides.
func (s riskSignal) mayMatch(folded string) bool {
	if s.literals == nil {
		return true
	}
	for _, lit := range s.literals {
		if strings.Contains(folded, lit) {
			return true
		}
	}
	return false
}

// RiskCategoryNames returns the set of known risk category names, so a
// configuration can be validated against the table without duplicating it.
func RiskCategoryNames() map[string]bool {
	out := make(map[string]bool, len(RawRiskSignals))
	for _, r := range RawRiskSignals {
		out[r.Category] = true
	}
	return out
}

// riskMemo holds the result for the last prompt. One routing call scans the
// same full prompt from the risk floor, Route's risk list, the semantic fuse
// and the NB guard; the scan is linear in a prompt of up to 1 MiB, so it runs
// once per prompt. Comparing equal strings costs far less than the scan.
var riskMemo struct {
	sync.Mutex
	prompt string
	cats   []string
	ok     bool
}

// RiskCategories returns the sorted risk categories the prompt touches.
func RiskCategories(prompt string) []string {
	riskMemo.Lock()
	if riskMemo.ok && riskMemo.prompt == prompt {
		cats := slices.Clone(riskMemo.cats)
		riskMemo.Unlock()
		return cats
	}
	riskMemo.Unlock()
	cats := scanRiskCategories(prompt)
	riskMemo.Lock()
	riskMemo.prompt, riskMemo.cats, riskMemo.ok = prompt, slices.Clone(cats), true
	riskMemo.Unlock()
	return cats
}

// parallelScanMin is the text size from which the candidate regexes run
// concurrently. Go's regexp is an NFA at a few MB/s per pattern, so a 1 MiB
// prompt whose literals defeat the prefilter spends ~200 ms per category;
// below this size the goroutine overhead outweighs the gain.
const parallelScanMin = 64 << 10

func scanRiskCategories(prompt string) []string {
	text := splitIdentifiers(prompt)
	folded := foldRiskText(text)
	var cand []riskSignal
	for _, s := range riskSignals() {
		if s.mayMatch(folded) {
			cand = append(cand, s)
		}
	}
	hit := make([]bool, len(cand))
	if len(text) >= parallelScanMin && len(cand) > 1 {
		var wg sync.WaitGroup
		for i := range cand {
			wg.Add(1)
			go func() {
				defer wg.Done()
				hit[i] = cand[i].re.MatchString(text)
			}()
		}
		wg.Wait()
	} else {
		for i := range cand {
			hit[i] = cand[i].re.MatchString(text)
		}
	}
	var cats []string
	for i, s := range cand {
		if hit[i] {
			cats = append(cats, s.category)
		}
	}
	sort.Strings(cats)
	return cats
}

// applyRiskAbstention clears confidence on known H14 buckets (bounded delegation
// briefs, M3 parallel high-stakes patterns) without changing complexity.
func applyRiskAbstention(prompt string, c Classification) Classification {
	if len(c.Risk) == 0 || !c.Confident || (c.Complexity != Medium && c.Complexity != Simple) {
		return c
	}
	wc := len(strings.Fields(prompt))
	if boundedBrief(prompt, wc) || (c.Complexity == Medium && riskHighStakesWorkRE.MatchString(prompt)) {
		c.Confident = false
		c.RiskAbstain = true
	}
	return c
}

// ApplyRiskFloor lifts a Trivial/Simple classification that touches a risk
// category to Medium (mid tier) and clears Confident, so no adapter acts on
// a confident downshift for it. applyRiskAbstention handles confident Medium/Simple
// with risk where the tier should stay unchanged.
func ApplyRiskFloor(prompt string, c Classification) Classification {
	cats := RiskCategories(prompt)
	if len(cats) > 0 {
		c.Risk = cats
		c = applyRiskAbstention(prompt, c)
	}
	if c.Complexity >= Medium {
		return c
	}
	if len(cats) == 0 {
		return c
	}
	c.Complexity = Medium
	c.Confident = false
	c.RiskFloor = true
	return c
}
