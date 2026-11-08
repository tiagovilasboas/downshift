// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

const (
	helperStoreEnv = "LEARNING_TEST_STORE"
	helperCountEnv = "LEARNING_TEST_COUNT"
)

var testArgv = []string{"zebra", "audit"}

// TestHelperRecordLoop is the body of the child processes below; it does
// nothing in a normal test run.
func TestHelperRecordLoop(t *testing.T) {
	p := os.Getenv(helperStoreEnv)
	if p == "" {
		t.Skip("child process only")
	}
	n, _ := strconv.Atoi(os.Getenv(helperCountEnv))
	SetStorePath(p)
	lockWait = 30 * time.Second // a loaded machine must not skip in this test
	for i := 0; i < n; i++ {
		if err := Record(testArgv, []byte("line\n"), ""); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordAcrossProcessesLosesNoUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), StoreFileName)
	const procs, each = 4, 15
	var wg sync.WaitGroup
	for i := 0; i < procs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperRecordLoop$")
			cmd.Env = append(os.Environ(), "DOWNSHIFT_STATE_DIR="+filepath.Dir(path), helperStoreEnv+"="+path, helperCountEnv+"="+strconv.Itoa(each))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("child: %v\n%s", err, out)
			}
		}()
	}
	wg.Wait()
	SetStorePath(path)
	defer SetStorePath("")
	st, err := Export()
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, e := range st.Entries {
		total += e.Count
	}
	if total != procs*each {
		t.Fatalf("recorded %d, want %d (lost updates)", total, procs*each)
	}
}

func TestRecordSkipsWhenLockBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), StoreFileName)
	SetStorePath(path)
	defer SetStorePath("")
	old := lockWait
	lockWait = 50 * time.Millisecond
	defer func() { lockWait = old }()

	held, err := telemetry.LockFile(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := Record(testArgv, []byte("x\n"), ""); err != nil {
		t.Fatalf("busy lock must fail open, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("Record blocked on a busy lock")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("recorded despite the held lock")
	}
	_ = held.Unlock()
	if err := Record(testArgv, []byte("x\n"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("not recorded after unlock")
	}
}
