package task

func CountEven(xs []int) int {
	n := 0
	for _, x := range xs {
		if x%2 == 0 {
			n++
		}
	}
	return n
}
