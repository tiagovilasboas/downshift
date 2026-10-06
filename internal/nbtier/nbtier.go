// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package nbtier is a Go port of tools/baseline/nb_tier.py: a multinomial
// naive Bayes over lowercase word unigrams and bigrams (Laplace alpha 1,
// uniform prior), trained once on an embedded copy of benchmark/tasks.json.
// It is only ever used as an upshift-only second opinion (see
// docs/design/router-generalization.md); it never lowers a tier.
package nbtier

import (
	_ "embed"
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"sync"
)

// Margin is the upshift threshold frozen on seed + holdout on 2026-10-03
// (docs/design/router-generalization.md). Do not tune it on eval-only splits.
const Margin = 0.10

// Tiers: 0 small, 1 mid, 2 frontier.
const (
	Small = iota
	Mid
	Frontier
)

//go:embed train.json
var trainJSON []byte

var labels = []string{"TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"}

var labelTier = map[string]int{"TRIVIAL": Small, "SIMPLE": Small, "MEDIUM": Mid, "COMPLEX": Frontier}

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

type model struct {
	logp   [4]map[string]float64 // log P(feature | label) for features seen with the label
	unseen [4]float64            // log P(feature | label) for vocabulary features not seen with it
	vocab  map[string]bool
}

var (
	once sync.Once
	nb   *model
)

func feats(text string) []string {
	w := wordRe.FindAllString(strings.ToLower(text), -1)
	out := append([]string{}, w...)
	for i := 0; i+1 < len(w); i++ {
		out = append(out, w[i]+"_"+w[i+1])
	}
	return out
}

func load() *model {
	once.Do(func() {
		var rows []struct{ Prompt, Label string }
		if err := json.Unmarshal(trainJSON, &rows); err != nil {
			panic("nbtier: embedded train.json: " + err.Error())
		}
		var counts [4]map[string]int
		for i := range counts {
			counts[i] = map[string]int{}
		}
		m := &model{vocab: map[string]bool{}}
		for _, r := range rows {
			li := labelIndex(r.Label)
			for _, f := range feats(r.Prompt) {
				counts[li][f]++
				m.vocab[f] = true
			}
		}
		for li := range labels {
			total := 0
			for _, c := range counts[li] {
				total += c
			}
			denom := float64(total) + float64(len(m.vocab))
			m.logp[li] = make(map[string]float64, len(counts[li]))
			for f, c := range counts[li] {
				m.logp[li][f] = math.Log((float64(c) + 1) / denom)
			}
			m.unseen[li] = math.Log(1 / denom)
		}
		nb = m
	})
	return nb
}

func labelIndex(l string) int {
	for i, x := range labels {
		if x == l {
			return i
		}
	}
	panic("nbtier: unknown label " + l)
}

// scores returns the per-label log-likelihood and the number of known features.
func scores(prompt string) ([4]float64, int) {
	m := load()
	var ll [4]float64
	n := 0
	for _, f := range feats(prompt) {
		if !m.vocab[f] {
			continue
		}
		n++
		for li := range labels {
			if v, ok := m.logp[li][f]; ok {
				ll[li] += v
			} else {
				ll[li] += m.unseen[li]
			}
		}
	}
	return ll, n
}

// Predict returns the NB label; MEDIUM when no feature is known.
func Predict(prompt string) string {
	ll, n := scores(prompt)
	if n == 0 {
		return "MEDIUM"
	}
	best := 0
	for li := 1; li < len(labels); li++ {
		if ll[li] > ll[best] {
			best = li
		}
	}
	return labels[best]
}

// Suggestion is NB's upshift opinion for one prompt.
type Suggestion struct {
	From, To int     // production tier and NB tier
	Margin   float64 // per-feature log-likelihood gap between the two tiers
	Upshift  bool    // To > From and Margin >= the frozen threshold
}

// Suggest compares NB with the production tier. It never suggests a lower tier.
func Suggest(prompt string, prodTier int) Suggestion {
	s := Suggestion{From: prodTier, To: prodTier}
	nbTier := labelTier[Predict(prompt)]
	if nbTier <= prodTier {
		return s
	}
	ll, n := scores(prompt)
	best := func(t int) float64 {
		b := math.Inf(-1)
		for li, l := range labels {
			if labelTier[l] == t && ll[li] > b {
				b = ll[li]
			}
		}
		return b
	}
	s.To = nbTier
	if n > 0 { // no known feature: NB fell back to MEDIUM with margin 0
		s.Margin = (best(nbTier) - best(prodTier)) / float64(n)
	}
	s.Upshift = s.Margin >= Margin
	return s
}

// DownshiftMargin is the small-vs-rest margin a TRIVIAL opinion needs before
// it may be used as a downshift. Chosen on the tuning prompts only
// (docs/design/router-generalization.md, "Small over-routing").
const DownshiftMargin = 0.20

// Trivial reports whether NB labels the prompt TRIVIAL with a small-vs-rest
// margin of at least DownshiftMargin. The margin is the per-feature
// log-likelihood gap between the best small-tier label and the best
// mid/frontier label. No known feature means no opinion.
func Trivial(prompt string) (bool, float64) {
	ll, n := scores(prompt)
	if n == 0 {
		return false, 0
	}
	margin := (math.Max(ll[0], ll[1]) - math.Max(ll[2], ll[3])) / float64(n)
	return Predict(prompt) == "TRIVIAL" && margin >= DownshiftMargin, margin
}
