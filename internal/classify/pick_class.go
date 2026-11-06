// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

// pickClass returns the top-scoring class and whether it beat the runner-up
// by a clear margin. Ties favour the higher complexity.
func pickClass(scores map[Complexity]int) (Complexity, bool) {
	order := []Complexity{Complex, Medium, Simple, Trivial}
	top := Medium // safe default when nothing matches
	topScore := 0
	for _, c := range order {
		if scores[c] > topScore {
			topScore = scores[c]
			top = c
		}
	}
	second := 0
	for _, c := range order {
		if c == top {
			continue
		}
		if scores[c] > second {
			second = scores[c]
		}
	}
	return top, topScore > 0 && topScore-second >= 2
}

// focusOnTask chooses between the whole-brief result and the task-focused
// tally of a long brief. The focus may lower the class or its confidence; it
// never spends money on doubt:
//   - it never raises the class, and never makes a Complex result confident;
//   - an unconfident lower class only clears confidence on the whole-brief
//     class, so the router keeps the requested model.
func focusOnTask(base Classification, focus map[Complexity]int) Classification {
	top, confident := pickClass(focus)
	if base.UnsupportedLanguage {
		confident = false
	}
	switch {
	case top > base.Complexity,
		top == Complex && confident && !base.Confident,
		top == base.Complexity && confident == base.Confident:
		return base
	case top < base.Complexity && !confident:
		base.Confident = false
		base.TaskFocus = true
		return base
	}
	return Classification{
		Complexity:          top,
		Scores:              focus,
		Confident:           confident,
		UnsupportedLanguage: base.UnsupportedLanguage,
		TaskFocus:           true,
	}
}
