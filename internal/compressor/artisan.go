// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Laravel/Lumen Artisan command output compressor.
// Handles: migrate, route:list, config:show, queue:work, etc.

var (
	// artisan migrate patterns
	reArtisanMigrating     = regexp.MustCompile(`(?m)^\s*Migrating:\s+(.+)$`)
	reArtisanMigrated      = regexp.MustCompile(`(?m)^\s*Migrated:\s+(.+)\s+\([\d.]+ms\)$`)
	reArtisanNothingMigrate = regexp.MustCompile(`(?m)Nothing to migrate`)
	reArtisanRollingBack   = regexp.MustCompile(`(?m)^\s*Rolling back:\s+`)
	
	// artisan route:list patterns (table with lots of columns)
	reArtisanRouteHeader   = regexp.MustCompile(`(?m)^\s*(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\|`)
	reArtisanRouteDomain   = regexp.MustCompile(`(?m)^\s*Domain\s+\|\s+Method\s+\|`)
	
	// artisan general patterns
	reArtisanInfo          = regexp.MustCompile(`(?m)^\s*INFO\s+`)
	reArtisanWarn          = regexp.MustCompile(`(?m)^\s*WARN\s+`)
	reArtisanError         = regexp.MustCompile(`(?m)^\s*ERROR\s+|Exception|SQLSTATE`)
)

// isArtisanMigrate detects artisan migrate output.
func isArtisanMigrate(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return reArtisanMigrating.Match(input) || 
		reArtisanMigrated.Match(input) || 
		reArtisanNothingMigrate.Match(input) ||
		reArtisanRollingBack.Match(input)
}

// isArtisanRouteList detects artisan route:list output.
func isArtisanRouteList(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return reArtisanRouteHeader.Match(input) || reArtisanRouteDomain.Match(input)
}

// compressArtisanMigrate compacts migration output.
func compressArtisanMigrate(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "artisan_migrate_too_large"
	}

	// Fail open on errors
	if reArtisanError.Match(input) {
		return input, false, "artisan_migrate_has_error"
	}

	// Nothing to migrate
	if reArtisanNothingMigrate.Match(input) {
		return []byte("✓ artisan migrate: nothing to migrate\n"), true, "artisan_migrate_nothing"
	}

	lines := strings.Split(string(input), "\n")
	var migrations []string
	var rollbacks []string

	for _, line := range lines {
		if m := reArtisanMigrated.FindStringSubmatch(line); m != nil {
			migrations = append(migrations, m[1])
		}
		if reArtisanRollingBack.MatchString(line) {
			rollbacks = append(rollbacks, line)
		}
	}

	if len(migrations) == 0 && len(rollbacks) == 0 {
		return input, false, "artisan_migrate_no_ops"
	}

	var out bytes.Buffer
	if len(migrations) > 0 {
		out.WriteString("✓ artisan migrate: ")
		out.WriteString(itoa(len(migrations)))
		out.WriteString(" migrations applied\n")
		// Show last 3 migration names
		start := 0
		if len(migrations) > 3 {
			start = len(migrations) - 3
			out.WriteString("  ... and ")
			out.WriteString(itoa(len(migrations)-3))
			out.WriteString(" more\n")
		}
		for _, m := range migrations[start:] {
			out.WriteString("  • ")
			out.WriteString(m)
			out.WriteString("\n")
		}
	}

	if len(rollbacks) > 0 {
		out.WriteString("↩ artisan migrate:rollback: ")
		out.WriteString(itoa(len(rollbacks)))
		out.WriteString(" rolled back\n")
	}

	return out.Bytes(), true, "artisan_migrate_compacted"
}

// compressArtisanRouteList compacts route:list output.
func compressArtisanRouteList(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "artisan_routes_too_large"
	}

	lines := strings.Split(string(input), "\n")
	
	var getCount, postCount, putCount, deleteCount, otherCount int
	var middlewareSet = make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "|") && strings.Contains(line, "Method") {
			continue
		}

		upper := strings.ToUpper(line)
		switch {
		case strings.Contains(upper, "GET"):
			getCount++
		case strings.Contains(upper, "POST"):
			postCount++
		case strings.Contains(upper, "PUT") || strings.Contains(upper, "PATCH"):
			putCount++
		case strings.Contains(upper, "DELETE"):
			deleteCount++
		default:
			if strings.Contains(line, "|") {
				otherCount++
			}
		}

		// Extract middleware
		if strings.Contains(line, "auth") {
			middlewareSet["auth"] = true
		}
		if strings.Contains(line, "api") {
			middlewareSet["api"] = true
		}
		if strings.Contains(line, "web") {
			middlewareSet["web"] = true
		}
	}

	total := getCount + postCount + putCount + deleteCount + otherCount
	if total == 0 {
		return input, false, "artisan_routes_no_routes"
	}

	var out bytes.Buffer
	out.WriteString("✓ artisan route:list: ")
	out.WriteString(itoa(total))
	out.WriteString(" routes\n")
	out.WriteString("  GET: ")
	out.WriteString(itoa(getCount))
	out.WriteString(" | POST: ")
	out.WriteString(itoa(postCount))
	out.WriteString(" | PUT/PATCH: ")
	out.WriteString(itoa(putCount))
	out.WriteString(" | DELETE: ")
	out.WriteString(itoa(deleteCount))
	out.WriteString("\n")

	if len(middlewareSet) > 0 {
		out.WriteString("  middleware: ")
		var mw []string
		for k := range middlewareSet {
			mw = append(mw, k)
		}
		out.WriteString(strings.Join(mw, ", "))
		out.WriteString("\n")
	}

	return out.Bytes(), true, "artisan_routes_compacted"
}

// FormatArtisanMigrate constant
const FormatArtisanMigrate Format = "artisan_migrate"

// FormatArtisanRoutes constant  
const FormatArtisanRoutes Format = "artisan_routes"
