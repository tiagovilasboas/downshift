// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"fmt"
	"strings"
)

// platforms are the GOOS/GOARCH pairs goreleaser publishes (.goreleaser.yaml).
var platforms = map[string]bool{
	"darwin/amd64": true, "darwin/arm64": true,
	"linux/amd64": true, "linux/arm64": true,
	"windows/amd64": true,
}

// ArchiveName is the release asset for a tag and platform, e.g.
// downshift_0.14.0_darwin_arm64.tar.gz. Windows archives are zip files.
func ArchiveName(tag, goos, goarch string) (string, error) {
	if !platforms[goos+"/"+goarch] {
		return "", fmt.Errorf("no release archive for %s/%s; build from source instead", goos, goarch)
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("downshift_%s_%s_%s%s", strings.TrimPrefix(tag, "v"), goos, goarch, ext), nil
}

// BinaryName is the executable inside the archive.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "downshift.exe"
	}
	return "downshift"
}
