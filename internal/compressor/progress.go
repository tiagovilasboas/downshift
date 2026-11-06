// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"strings"
)

// FormatProgress marks output whose only change is that carriage-return
// progress frames were collapsed to the frame a terminal would have left on
// screen. It is never detected from the text; CompressExit reports it only
// when the collapse shortened the output and no other format applied.
const FormatProgress Format = "progress"

// collapseProgressText is collapseProgress over the text screenText would
// read: SGR colour is removed first, any other escape keeps the bytes raw.
func collapseProgressText(input []byte) ([]byte, bool) {
	if bytes.IndexByte(input, 0x1b) != -1 {
		plain, ok := stripColour(input)
		if !ok {
			return nil, false
		}
		input = plain
	}
	return collapseProgress(input)
}

// collapseProgress replaces every line of "frame\rframe\r...frame" by its
// last frame. It returns false, leaving the caller on the raw bytes, unless
// the collapse is provably what a terminal shows:
//
//   - the bytes are printable ASCII, "\n" and "\r" only. No escape sequence
//     (a cursor move or line erase changes what is on screen), no tab, and no
//     wide or combining character, so one byte is one column;
//   - no CRLF line ending: a "\r" before "\n" is a line ending, not a frame;
//   - each frame is at least as long as the one before it. A shorter frame
//     leaves the tail of the previous one visible ("abcdef\rxy" shows
//     "xycdef"), so the final frame alone would lose text;
//   - no danger word, stack frame or failure marker anywhere, in any frame
//     (genericHasDanger), because the earlier frames are the part cut.
//
// A line without "\r" is untouched. It reports false when nothing collapsed.
func collapseProgress(input []byte) ([]byte, bool) {
	for _, c := range input {
		if c >= 0x80 || (c < 0x20 && c != '\n' && c != '\r') || c == 0x7f {
			return nil, false
		}
	}
	text := string(input)
	if strings.Contains(text, "\r\n") || genericHasDanger(text) {
		return nil, false
	}
	lines := strings.Split(text, "\n")
	collapsed := false
	for i, line := range lines {
		if !strings.Contains(line, "\r") {
			continue
		}
		frames := strings.Split(line, "\r")
		for j := 1; j < len(frames); j++ {
			if len(frames[j]) < len(frames[j-1]) {
				return nil, false
			}
		}
		lines[i] = frames[len(frames)-1]
		collapsed = true
	}
	if !collapsed {
		return nil, false
	}
	return []byte(strings.Join(lines, "\n")), true
}
