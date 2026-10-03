package task

import (
	"sort"
)

func Merge(intervals [][2]int) [][2]int {
	xs := append([][2]int(nil), intervals...)
	sort.Slice(xs, func(i, j int) bool { return xs[i][0] < xs[j][0] })
	var out [][2]int
	for _, iv := range xs {
		if n := len(out); n > 0 && iv[0] <= out[n-1][1] {
			if iv[1] > out[n-1][1] {
				out[n-1][1] = iv[1]
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}
