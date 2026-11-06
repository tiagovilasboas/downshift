// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"encoding/json"
	"os"
)

// DefaultMaxWorkersPerSession is how many worker jobs one server session may
// start in total. 0 in the policy file means no cap.
const DefaultMaxWorkersPerSession = 32

// MaxWorkersPerSession reads orchestration.max_workers_per_session from the
// user file, falling back to the default, then lets the project file lower it:
// a project value counts only when it is above 0 and below the user's (a repo
// can never remove or raise the cap). A missing, unreadable or invalid value
// (negative, not an integer) holds no value. The key sits in the orchestration
// section parallel.go reads, so the routing schema is untouched.
func MaxWorkersPerSession(loc Locations) int {
	limit := DefaultMaxWorkersPerSession
	if n, ok := fileWorkersCap(loc.User); ok {
		limit = n
	}
	if n, ok := fileWorkersCap(loc.Project); ok && n > 0 && (limit == 0 || n < limit) {
		limit = n
	}
	return limit
}

func fileWorkersCap(path string) (int, bool) {
	if path == "" {
		return 0, false
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxBytes {
		return 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var doc struct {
		Orchestration struct {
			Cap *int `json:"max_workers_per_session"`
		} `json:"orchestration"`
	}
	if json.Unmarshal(data, &doc) != nil || doc.Orchestration.Cap == nil || *doc.Orchestration.Cap < 0 {
		return 0, false
	}
	return *doc.Orchestration.Cap, true
}
