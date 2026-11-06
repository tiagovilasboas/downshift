// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin

package setup

// platformOwnerAppState cannot tell on this OS, so callers print the warning
// as a note.
func platformOwnerAppState(string) OwnerAppState { return OwnerAppUnknown }
