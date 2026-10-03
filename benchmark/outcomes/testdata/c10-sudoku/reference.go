package task

func ok(b *[9][9]int, r, c, v int) bool {
	for i := 0; i < 9; i++ {
		if b[r][i] == v || b[i][c] == v || b[r/3*3+i/3][c/3*3+i%3] == v {
			return false
		}
	}
	return true
}

func solve(b *[9][9]int) bool {
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if b[r][c] != 0 {
				continue
			}
			for v := 1; v <= 9; v++ {
				if ok(b, r, c, v) {
					b[r][c] = v
					if solve(b) {
						return true
					}
					b[r][c] = 0
				}
			}
			return false
		}
	}
	return true
}

func Solve(board [9][9]int) ([9][9]int, bool) {
	b := board
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			v := b[r][c]
			if v < 0 || v > 9 {
				return board, false
			}
			if v != 0 {
				b[r][c] = 0
				if !ok(&b, r, c, v) {
					return board, false
				}
				b[r][c] = v
			}
		}
	}
	if !solve(&b) {
		return board, false
	}
	return b, true
}
