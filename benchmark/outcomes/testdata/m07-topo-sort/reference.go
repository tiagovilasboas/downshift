package task

import (
	"errors"
	"sort"
)

func TopoSort(deps map[string][]string) ([]string, error) {
	indeg := map[string]int{}
	users := map[string][]string{}
	for n, ds := range deps {
		if _, ok := indeg[n]; !ok {
			indeg[n] = 0
		}
		for _, d := range ds {
			if _, ok := indeg[d]; !ok {
				indeg[d] = 0
			}
			indeg[n]++
			users[d] = append(users[d], n)
		}
	}
	var ready, out []string
	for n, d := range indeg {
		if d == 0 {
			ready = append(ready, n)
		}
	}
	for len(ready) > 0 {
		sort.Strings(ready)
		n := ready[0]
		ready = ready[1:]
		out = append(out, n)
		for _, u := range users[n] {
			indeg[u]--
			if indeg[u] == 0 {
				ready = append(ready, u)
			}
		}
	}
	if len(out) != len(indeg) {
		return nil, errors.New("cycle detected")
	}
	return out, nil
}
