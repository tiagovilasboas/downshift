// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package selfupdate replaces the running downshift binary with a published
// release: it resolves the tag, downloads the archive for this platform and
// checksums.txt, checks the sha256, extracts the binary and swaps it in place.
// It never runs setup or writes harness files; `downshift update` does that
// with the new binary. Every download is https only.
package selfupdate
