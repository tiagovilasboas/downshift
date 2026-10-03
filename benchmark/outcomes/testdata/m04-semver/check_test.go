package task

import (
	"testing"
)

func TestCompareSemver(t *testing.T) {
	order := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.2.0", "1.10.0", "2.0.0"}
	for i := range order {
		for j := range order {
			got, err := CompareSemver(order[i], order[j])
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if err != nil || got != want {
				t.Errorf("Compare(%s,%s)=%d,%v want %d", order[i], order[j], got, err, want)
			}
		}
	}
	for _, bad := range []string{"1.0", "1.0.x", "", "1.0.0-", "a.b.c"} {
		if _, err := CompareSemver(bad, "1.0.0"); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}
