// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DiscoveredFile represents a source file that passed discovery boundaries and security filters.
type DiscoveredFile struct {
	RelativePath string
	FullPath     string
	Size         int64
	ModTime      time.Time
}

// Default binary extensions to exclude from indexing.
var binaryExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".bin":   true,
	".iso":   true,
	".img":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
	".bz2":   true,
	".xz":    true,
	".7z":    true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".ico":   true,
	".webp":  true,
	".pdf":   true,
	".doc":   true,
	".docx":  true,
	".ppt":   true,
	".pptx":  true,
	".xls":   true,
	".xlsx":  true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
	".mp3":   true,
	".mp4":   true,
	".wav":   true,
	".avi":   true,
	".mov":   true,
	".pyc":   true,
	".class": true,
	".o":     true,
	".a":     true,
}

// DiscoverFiles discovers and filters valid source code files within repoRoot using git ls-files.
// It enforces bounded memory consumption, file counts, sizes, symlink safety, and exclusion lists.
func DiscoverFiles(ctx context.Context, repoRoot string, cfg Config) ([]DiscoveredFile, error) {
	if repoRoot == "" {
		return nil, errors.New("empty repository root")
	}

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}

	// Verify repoRoot exists and is a directory
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("stat repo root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repo root is not a directory: %s", absRoot)
	}

	// Execute git ls-files --cached --others --exclude-standard -z
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = absRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	var rawEntries [][]byte
	if err := cmd.Run(); err == nil {
		rawEntries = bytes.Split(stdout.Bytes(), []byte{0})
	} else {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Fallback for non-git directories (e.g. temporary test directories)
		_ = filepath.WalkDir(absRoot, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || (strings.HasPrefix(name, ".") && name != ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if rel, err := filepath.Rel(absRoot, p); err == nil {
				rawEntries = append(rawEntries, []byte(rel))
			}
			return nil
		})
	}

	maxFiles := cfg.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 2000
	}
	maxFileSize := cfg.MaxFileSizeBytes
	if maxFileSize <= 0 {
		maxFileSize = 512 * 1024 // 512 KiB
	}
	maxRunBytes := cfg.MaxRunBytes
	if maxRunBytes <= 0 {
		maxRunBytes = 16 * 1024 * 1024 // 16 MiB
	}

	excludes := cfg.Excludes
	if len(excludes) == 0 {
		excludes = DefaultConfig().Excludes
	}

	var totalBytes int64
	var discovered []DiscoveredFile

	for _, entry := range rawEntries {
		if len(entry) == 0 {
			continue
		}

		relPath := string(entry)

		// Security: clean and ensure no path traversal
		cleanRel := filepath.Clean(relPath)
		if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			continue
		}

		// Check exclusion patterns
		if isExcluded(cleanRel, excludes) {
			continue
		}

		// Check sensitive files and secrets
		if isSensitive(cleanRel) {
			continue
		}

		// Check binary extension
		ext := strings.ToLower(filepath.Ext(cleanRel))
		if binaryExtensions[ext] {
			continue
		}

		fullPath := filepath.Join(absRoot, cleanRel)

		// Symlink safety: use Lstat to reject symlinks
		lfi, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		if lfi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if !lfi.Mode().IsRegular() {
			continue
		}

		// Enforce single file size limit
		fileSize := lfi.Size()
		if fileSize > maxFileSize || fileSize == 0 {
			continue
		}

		// Enforce cumulative run budget
		if totalBytes+fileSize > maxRunBytes {
			break
		}

		// Fast check for binary content (null byte in first 512 bytes)
		if isBinaryContent(fullPath) {
			continue
		}

		discovered = append(discovered, DiscoveredFile{
			RelativePath: filepath.ToSlash(cleanRel),
			FullPath:     fullPath,
			Size:         fileSize,
			ModTime:      lfi.ModTime(),
		})
		totalBytes += fileSize

		if len(discovered) >= maxFiles {
			break
		}
	}

	return discovered, nil
}

// isExcluded returns true if relPath matches any excluded directory or file prefix.
func isExcluded(relPath string, excludes []string) bool {
	norm := filepath.ToSlash(relPath)
	parts := strings.Split(norm, "/")

	for _, exclude := range excludes {
		normExclude := filepath.ToSlash(exclude)
		for _, part := range parts {
			if part == normExclude {
				return true
			}
		}
		if strings.HasPrefix(norm, normExclude+"/") || norm == normExclude {
			return true
		}
	}
	return false
}

// isSensitive detects potential secrets, tokens, credentials, and environment files.
func isSensitive(relPath string) bool {
	return IsSecretFile(relPath)
}

// isBinaryContent checks the first 512 bytes of a file for null bytes.
func isBinaryContent(fullPath string) bool {
	f, err := os.Open(fullPath)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}

	return bytes.Contains(buf[:n], []byte{0})
}
