// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package lazyre

import (
	"sync"
	"testing"
)

func TestCompilesOnFirstUseOnly(t *testing.T) {
	r := New(`^a(b+)$`)
	if r.re != nil {
		t.Fatal("New compiled the expression; it must wait for first use")
	}
	if r.String() != `^a(b+)$` || r.re != nil {
		t.Fatal("String must return the source without compiling")
	}
	if !r.MatchString("abb") || r.re == nil {
		t.Fatal("MatchString must compile and match")
	}
	first := r.Get()
	if r.Get() != first {
		t.Fatal("Get must compile once")
	}
}

func TestForwardsMatchMethods(t *testing.T) {
	r := New(`(\d+)-(\d+)`)
	if got := r.FindStringSubmatch("x 12-34 y"); len(got) != 3 || got[2] != "34" {
		t.Fatalf("FindStringSubmatch = %v", got)
	}
	if got := r.ReplaceAllString("1-2 3-4", "$2-$1"); got != "2-1 4-3" {
		t.Fatalf("ReplaceAllString = %q", got)
	}
	if got := r.FindAllString("1-2 3-4", -1); len(got) != 2 {
		t.Fatalf("FindAllString = %v", got)
	}
	if got := New(`,\s*`).Split("a, b,c", -1); len(got) != 3 {
		t.Fatalf("Split = %v", got)
	}
	if !r.Match([]byte("7-8")) || string(r.ReplaceAll([]byte("7-8"), []byte("$1"))) != "7" {
		t.Fatal("byte methods must forward")
	}
}

func TestConcurrentFirstUse(t *testing.T) {
	r := New(`x+`)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			if !r.MatchString("xx") {
				t.Error("no match")
			}
		})
	}
	wg.Wait()
}

func TestCompileAllPanicsOnInvalidExpression(t *testing.T) {
	bad := New(`(`)
	defer func() {
		registryMu.Lock()
		registry = registry[:len(registry)-1] // drop bad so later calls pass
		registryMu.Unlock()
		if recover() == nil {
			t.Fatal("CompileAll must panic on an invalid expression")
		}
	}()
	_ = bad
	CompileAll()
}
