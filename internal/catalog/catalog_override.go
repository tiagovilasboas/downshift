// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// applyUserOverride merges the user catalog into base. A user catalog is an
// override, not a replacement: it preserves all embedded entries it does not
// name and replaces only matching harness/model IDs. The file is untrusted
// input read on every hook, so any failure, even a panic in the merge, leaves
// base untouched (embedded catalog) and only warns.
func applyUserOverride(base *catalogFile, warning io.Writer) {
	path, ok := userCatalogPath()
	if !ok {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(warning, "downshift: warning: could not apply %s (%v); using embedded catalog\n", path, r)
		}
	}()
	data, err := readUserCatalog(path)
	if err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not read %s (%v); using embedded catalog\n", path, err)
		return
	}
	override, err := decode(data)
	if err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not parse %s (%v); using embedded catalog\n", path, err)
		return
	}
	merged := mergeEntries(base.Entries, override.Entries)
	if err := validateEntries(merged); err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not merge %s (%v); using embedded catalog\n", path, err)
		return
	}
	base.Entries = merged
	base.SessionCandidates = mergeSessionCandidates(base.SessionCandidates, override.SessionCandidates)
}

// maxOpenRouterAllowlistBytes caps the OpenRouter allowlist read.
const maxOpenRouterAllowlistBytes = 256 << 10

// applyOpenRouterAllowlist adds the rows the user listed in openrouter.json.
// A missing file means no allowlist and is silent. Any other failure leaves
// base untouched and only warns, so a bad file never blocks a spawn. The
// listed rows go ahead of the catalog rows so an explicit user choice wins
// the tier.
func applyOpenRouterAllowlist(base *catalogFile, warning io.Writer) {
	path, err := paths.GatewayAllowlistPath()
	if err != nil {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(warning, "downshift: warning: could not apply %s (%v); ignoring it\n", path, r)
		}
	}()
	data, err := readCapped(path, maxOpenRouterAllowlistBytes)
	if err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not read %s (%v); ignoring it\n", path, err)
		return
	}
	rows, err := parseOpenRouterAllowlist(data)
	if err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not parse %s (%v); ignoring it\n", path, err)
		return
	}
	merged := append(rows, base.Entries...)
	if err := validateEntries(merged); err != nil {
		fmt.Fprintf(warning, "downshift: warning: could not merge %s (%v); ignoring it\n", path, err)
		return
	}
	base.Entries = merged
}

// maxUserCatalogBytes caps the user override read: the file is untrusted
// input on every hook invocation.
const maxUserCatalogBytes = 4 << 20

// readUserCatalog reads the override only if it is a regular file, and never
// more than maxUserCatalogBytes. A FIFO, device or huge file is an error (the
// caller falls back to the embedded catalog) instead of a hang or an OOM.
func readUserCatalog(path string) ([]byte, error) {
	return readCapped(path, maxUserCatalogBytes)
}

// readCapped reads a regular file of at most limit bytes. Used for every
// user-owned JSON file the hooks read.
func readCapped(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("larger than %d bytes", limit)
	}
	return data, nil
}

// userCatalogPath returns the path to the user override file and whether it
// exists. Returns false if XDG / home resolution fails.
func userCatalogPath() (string, bool) {
	p, err := paths.CatalogPath()
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}
