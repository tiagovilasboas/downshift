// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

// MaxShapeBytes bounds how much of an output is scanned for its shape.
// Outputs are already capped by the runner; this keeps the synchronous hook
// cheap on the largest ones.
const MaxShapeBytes = 64 * 1024

// Shape describes the structure of an output without any of its text. Every
// field is a count, a bucket or a hash: no line, word, path, URL or branch
// name from the output is ever stored. Line classes are independent
// predicates, so one line may count in several of them.
type Shape struct {
	// ScannedBytes is how much of the output the counts below cover
	// (at most MaxShapeBytes).
	ScannedBytes int `json:"scanned_bytes"`
	// Lines is the number of lines scanned.
	Lines int `json:"lines"`
	// Blank lines are empty after trimming.
	Blank int `json:"blank"`
	// Short lines have at most 20 bytes (and are not blank).
	Short int `json:"short"`
	// Long lines have more than 120 bytes.
	Long int `json:"long"`
	// Indented lines start with a space or a tab.
	Indented int `json:"indented"`
	// NumberHeavy lines are at least 30% digits (ignoring whitespace).
	NumberHeavy int `json:"number_heavy"`
	// PathLike lines contain a slash- or backslash-separated token and no URL.
	PathLike int `json:"path_like"`
	// URLLike lines contain a scheme such as https://.
	URLLike int `json:"url_like"`
	// StatusLike lines carry a status word (ok, PASS, FAIL, error, warning...).
	StatusLike int `json:"status_like"`
	// LineLengths buckets lines by byte length: 0-20, 21-80, 81-160, over 160.
	LineLengths [4]int `json:"line_lengths"`
	// Bytes buckets every scanned byte by class.
	Bytes ByteClasses `json:"bytes"`
	// FirstLineShape is a truncated sha256 of the first non-blank line's
	// character-class mask (letters, digits and spaces collapsed; punctuation
	// kept), so two outputs with the same header layout share it. The mask
	// itself is not stored.
	FirstLineShape string `json:"first_line_shape,omitempty"`
}

// ByteClasses is a histogram of bytes by class.
type ByteClasses struct {
	Letters int `json:"letters"`
	Digits  int `json:"digits"`
	Space   int `json:"space"`
	Punct   int `json:"punct"`
	Other   int `json:"other"`
}

var (
	urlRe    = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://`)
	pathRe   = regexp.MustCompile(`[\w.~-]*[/\\][\w.-]+`)
	statusRe = regexp.MustCompile(`(?i)\b(ok|pass|passed|fail|failed|failure|error|warn|warning|success|done)\b`)
)

// ShapeOf computes the privacy-safe Shape of an output.
func ShapeOf(output []byte) Shape {
	if len(output) > MaxShapeBytes {
		output = output[:MaxShapeBytes]
	}
	s := Shape{ScannedBytes: len(output)}
	if len(output) == 0 {
		return s
	}
	s.Bytes = countByteClasses(output)
	for _, line := range bytes.Split(bytes.TrimRight(output, "\n"), []byte("\n")) {
		s.addLine(line)
	}
	return s
}

func (s *Shape) addLine(line []byte) {
	s.Lines++
	s.LineLengths[lengthBucket(len(line))]++
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		s.Blank++
		return
	}
	if s.FirstLineShape == "" {
		s.FirstLineShape = shapeHash(line)
	}
	if len(line) <= 20 {
		s.Short++
	}
	if len(line) > 120 {
		s.Long++
	}
	if line[0] == ' ' || line[0] == '\t' {
		s.Indented++
	}
	if isNumberHeavy(trimmed) {
		s.NumberHeavy++
	}
	switch {
	case urlRe.Match(line):
		s.URLLike++
	case pathRe.Match(line):
		s.PathLike++
	}
	if statusRe.Match(line) {
		s.StatusLike++
	}
}

func lengthBucket(n int) int {
	switch {
	case n <= 20:
		return 0
	case n <= 80:
		return 1
	case n <= 160:
		return 2
	default:
		return 3
	}
}

func isNumberHeavy(line []byte) bool {
	digits, total := 0, 0
	for _, b := range line {
		if unicode.IsSpace(rune(b)) {
			continue
		}
		total++
		if b >= '0' && b <= '9' {
			digits++
		}
	}
	return total > 0 && digits*10 >= total*3
}

func countByteClasses(data []byte) ByteClasses {
	var c ByteClasses
	for _, b := range data {
		r := rune(b)
		switch {
		case unicode.IsLetter(r):
			c.Letters++
		case unicode.IsDigit(r):
			c.Digits++
		case unicode.IsSpace(r):
			c.Space++
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			c.Punct++
		default:
			c.Other++
		}
	}
	return c
}

// shapeHash hashes the character-class mask of a line: letters become "a",
// digits "0", whitespace "_", everything else stays. Only the hash leaves
// this function.
func shapeHash(line []byte) string {
	var mask strings.Builder
	for _, b := range line {
		r := rune(b)
		switch {
		case unicode.IsLetter(r):
			mask.WriteByte('a')
		case unicode.IsDigit(r):
			mask.WriteByte('0')
		case unicode.IsSpace(r):
			mask.WriteByte('_')
		default:
			mask.WriteByte(b)
		}
	}
	h := sha256.Sum256([]byte(mask.String()))
	return hex.EncodeToString(h[:8])
}
