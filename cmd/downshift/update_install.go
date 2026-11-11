// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/selfupdate"
)

// installRelease downloads tag for this platform, checks its sha256 against
// checksums.txt, optionally checks build provenance, and swaps the binary in
// place. It returns the path it wrote.
func installRelease(tag string, d updateDeps, out, errw io.Writer) (string, int) {
	name, err := selfupdate.ArchiveName(tag, d.GOOS, d.GOARCH)
	if err != nil {
		fmt.Fprintf(errw, "update: %v\n", err)
		return "", 1
	}
	fmt.Fprintf(out, "Downloading downshift %s (%s/%s)...\n", tag, d.GOOS, d.GOARCH)
	archive, err := d.Source.Download(tag, name)
	if err == nil {
		err = verifyArchive(d.Source, tag, name, archive)
	}
	if err != nil {
		fmt.Fprintf(errw, "update: %v\n", err)
		return "", 1
	}
	fmt.Fprintln(out, "Checksum verified (sha256).")
	if d.Attest != nil {
		if ok, note := d.Attest(archive, name); ok {
			fmt.Fprintln(out, "Build provenance verified (GitHub attestation).")
		} else {
			fmt.Fprintf(errw, "warning: %s; continuing with the sha256 check only\n", note)
		}
	}
	bin, err := selfupdate.ExtractBinary(archive, name, selfupdate.BinaryName(d.GOOS))
	if err != nil {
		fmt.Fprintf(errw, "update: %v\n", err)
		return "", 1
	}
	_ = os.Remove(selfupdate.OldCopy(d.Exe)) // a Windows leftover from the last update
	path, err := selfupdate.Replace(d.Exe, bin)
	if err != nil {
		fmt.Fprintf(errw, "update: %v\n", err)
		return "", 1
	}
	fmt.Fprintf(out, "Installed downshift %s at %s\n", tag, path)
	return path, 0
}

func verifyArchive(src releaseSource, tag, name string, archive []byte) error {
	list, err := src.Download(tag, "checksums.txt")
	if err != nil {
		return err
	}
	want, err := selfupdate.ExpectedSum(list, name)
	if err != nil {
		return err
	}
	return selfupdate.VerifySum(archive, want)
}

// ghAttest runs `gh attestation verify` when the gh CLI is installed, as
// install.sh does. Without gh the check is skipped with a note.
func ghAttest(archive []byte, name string) (bool, string) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return false, "build provenance not checked (the gh CLI is not installed)"
	}
	dir, err := os.MkdirTemp("", "downshift-update-")
	if err != nil {
		return false, "build provenance not checked (no temp dir)"
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, name)
	if os.WriteFile(path, archive, 0o600) != nil {
		return false, "build provenance not checked (no temp dir)"
	}
	if exec.Command(gh, "attestation", "verify", path, "--repo", selfupdate.Repo).Run() != nil {
		return false, "build provenance could not be verified (no attestation, gh not signed in, or a mismatch: run 'gh attestation verify <file> --repo " + selfupdate.Repo + "')"
	}
	return true, ""
}
