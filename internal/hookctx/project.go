// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookctx

import "sync"

var (
	projectMu  sync.RWMutex
	projectDir string
)

// SetProjectDir records the working directory of the project the current hook
// invocation belongs to and returns a function that restores the previous
// value. The shared hook runner sets it once per invocation, so layers below
// (for example the tier floor) never parse harness payloads themselves.
func SetProjectDir(dir string) (restore func()) {
	projectMu.Lock()
	prev := projectDir
	projectDir = dir
	projectMu.Unlock()
	return func() {
		projectMu.Lock()
		projectDir = prev
		projectMu.Unlock()
	}
}

// ProjectDir returns the directory set by SetProjectDir, or "" outside a hook
// invocation. An empty value means "no project context": callers must not fall
// back to the process directory, or offline tools such as benchmarks would pick
// up a project policy they should ignore.
func ProjectDir() string {
	projectMu.RLock()
	defer projectMu.RUnlock()
	return projectDir
}
