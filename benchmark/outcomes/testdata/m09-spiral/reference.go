package task

func Spiral(m [][]int) []int {
	out := []int{}
	if len(m) == 0 {
		return out
	}
	top, bottom, left, right := 0, len(m)-1, 0, len(m[0])-1
	for top <= bottom && left <= right {
		for j := left; j <= right; j++ {
			out = append(out, m[top][j])
		}
		top++
		for i := top; i <= bottom; i++ {
			out = append(out, m[i][right])
		}
		right--
		if top <= bottom {
			for j := right; j >= left; j-- {
				out = append(out, m[bottom][j])
			}
			bottom--
		}
		if left <= right {
			for i := bottom; i >= top; i-- {
				out = append(out, m[i][left])
			}
			left++
		}
	}
	return out
}
