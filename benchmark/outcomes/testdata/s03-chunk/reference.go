package task

func Chunk(xs []int, n int) [][]int {
	if n <= 0 {
		return nil
	}
	var out [][]int
	for i := 0; i < len(xs); i += n {
		end := i + n
		if end > len(xs) {
			end = len(xs)
		}
		out = append(out, xs[i:end])
	}
	return out
}
