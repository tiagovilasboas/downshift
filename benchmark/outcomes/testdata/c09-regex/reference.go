package task

func Match(pattern, s string) bool {
	memo := map[[2]int]bool{}
	seen := map[[2]int]bool{}
	var m func(i, j int) bool
	m = func(i, j int) bool {
		key := [2]int{i, j}
		if seen[key] {
			return memo[key]
		}
		var res bool
		if i == len(pattern) {
			res = j == len(s)
		} else {
			first := j < len(s) && (pattern[i] == '.' || pattern[i] == s[j])
			op := byte(0)
			if i+1 < len(pattern) {
				op = pattern[i+1]
			}
			switch op {
			case '*':
				res = m(i+2, j) || (first && m(i, j+1))
			case '+':
				res = first && (m(i+2, j+1) || m(i, j+1))
			case '?':
				res = m(i+2, j) || (first && m(i+2, j+1))
			default:
				res = first && m(i+1, j+1)
			}
		}
		seen[key], memo[key] = true, res
		return res
	}
	return m(0, 0)
}
