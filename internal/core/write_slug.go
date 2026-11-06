// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// SpawnWriteName is the string a hook writes into a spawn's model field. A
// session may still carry a name the harness's spawn tool rejects (catalog
// field rejected_name on the model's entry); the model's own write name is
// written instead. When that write name is itself the rejected one, there is
// nothing acceptable to write and the rewrite is held (ok false). A nil
// resolver means no catalog and so no policy: the name is written unchanged.
// A name the catalog does not know is also written unchanged; the rejected
// name is an alias of its own entry, so the lookup finds it first.
func SpawnWriteName(harness, writeName string, res Resolver) (string, bool) {
	if writeName == "" {
		return "", false
	}
	if res == nil {
		return writeName, true
	}
	m, ok := res.LookupByID(harness, writeName)
	if !ok || m.WriteName() == "" || m.RejectedName == "" {
		return writeName, true
	}
	canonical := m.WriteName()
	if canonical == m.RejectedName {
		return "", false
	}
	if writeName == m.RejectedName {
		return canonical, true
	}
	return writeName, true
}
