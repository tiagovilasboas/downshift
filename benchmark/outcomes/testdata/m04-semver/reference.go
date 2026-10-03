package task

import (
	"fmt"
	"strconv"
	"strings"
)

type ver struct {
	core [3]int
	pre  []string
}

func parse(s string) (ver, error) {
	var v ver
	main, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("invalid version %q", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" {
			return v, fmt.Errorf("invalid version %q", s)
		}
		v.core[i] = n
	}
	if hasPre {
		if pre == "" {
			return v, fmt.Errorf("invalid version %q", s)
		}
		v.pre = strings.Split(pre, ".")
		for _, id := range v.pre {
			if id == "" {
				return v, fmt.Errorf("invalid version %q", s)
			}
		}
	}
	return v, nil
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func CompareSemver(a, b string) (int, error) {
	va, err := parse(a)
	if err != nil {
		return 0, err
	}
	vb, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if c := cmpInt(va.core[i], vb.core[i]); c != 0 {
			return c, nil
		}
	}
	switch {
	case len(va.pre) == 0 && len(vb.pre) == 0:
		return 0, nil
	case len(va.pre) == 0:
		return 1, nil
	case len(vb.pre) == 0:
		return -1, nil
	}
	for i := 0; i < len(va.pre) && i < len(vb.pre); i++ {
		x, y := va.pre[i], vb.pre[i]
		nx, ex := strconv.Atoi(x)
		ny, ey := strconv.Atoi(y)
		switch {
		case ex == nil && ey == nil:
			if c := cmpInt(nx, ny); c != 0 {
				return c, nil
			}
		case ex == nil:
			return -1, nil
		case ey == nil:
			return 1, nil
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c, nil
			}
		}
	}
	return cmpInt(len(va.pre), len(vb.pre)), nil
}
