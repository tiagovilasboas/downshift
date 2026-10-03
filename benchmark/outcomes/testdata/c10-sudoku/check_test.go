package task

import (
	"testing"
)

func valid(b [9][9]int) bool {
	for i := 0; i < 9; i++ {
		var row, col, box [10]bool
		for j := 0; j < 9; j++ {
			r, c := b[i][j], b[j][i]
			x := b[i/3*3+j/3][i%3*3+j%3]
			if r < 1 || r > 9 || c < 1 || c > 9 || x < 1 || x > 9 || row[r] || col[c] || box[x] {
				return false
			}
			row[r], col[c], box[x] = true, true, true
		}
	}
	return true
}

func TestSolve(t *testing.T) {
	puzzle := [9][9]int{
		{5, 3, 0, 0, 7, 0, 0, 0, 0}, {6, 0, 0, 1, 9, 5, 0, 0, 0}, {0, 9, 8, 0, 0, 0, 0, 6, 0},
		{8, 0, 0, 0, 6, 0, 0, 0, 3}, {4, 0, 0, 8, 0, 3, 0, 0, 1}, {7, 0, 0, 0, 2, 0, 0, 0, 6},
		{0, 6, 0, 0, 0, 0, 2, 8, 0}, {0, 0, 0, 4, 1, 9, 0, 0, 5}, {0, 0, 0, 0, 8, 0, 0, 7, 9},
	}
	got, ok := Solve(puzzle)
	if !ok || !valid(got) {
		t.Fatalf("no valid solution: %v %v", ok, got)
	}
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if puzzle[r][c] != 0 && got[r][c] != puzzle[r][c] {
				t.Fatal("given digit changed")
			}
		}
	}
	bad := puzzle
	bad[0][2] = 5 // duplicate 5 in row 0
	if _, ok := Solve(bad); ok {
		t.Fatal("rule-breaking board must be rejected")
	}
	unsolvable := [9][9]int{}
	unsolvable[0] = [9]int{0, 2, 3, 4, 5, 6, 7, 8, 9}
	unsolvable[1][0] = 1 // the only candidate for (0,0) is 1, but column 0 already has it
	if _, ok := Solve(unsolvable); ok {
		t.Fatal("unsolvable board must be rejected")
	}
}
