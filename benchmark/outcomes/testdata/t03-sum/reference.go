package task

func Sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}
