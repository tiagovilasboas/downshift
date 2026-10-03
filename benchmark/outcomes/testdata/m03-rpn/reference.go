package task

import (
	"errors"
	"fmt"
	"strconv"
)

func EvalRPN(tokens []string) (int, error) {
	var st []int
	for _, tok := range tokens {
		switch tok {
		case "+", "-", "*", "/":
			if len(st) < 2 {
				return 0, errors.New("stack underflow")
			}
			a, b := st[len(st)-2], st[len(st)-1]
			st = st[:len(st)-2]
			var r int
			switch tok {
			case "+":
				r = a + b
			case "-":
				r = a - b
			case "*":
				r = a * b
			case "/":
				if b == 0 {
					return 0, errors.New("division by zero")
				}
				r = a / b
			}
			st = append(st, r)
		default:
			n, err := strconv.Atoi(tok)
			if err != nil {
				return 0, fmt.Errorf("bad token %q", tok)
			}
			st = append(st, n)
		}
	}
	if len(st) != 1 {
		return 0, errors.New("malformed expression")
	}
	return st[0], nil
}
