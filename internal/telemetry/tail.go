// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"io"
	"os"
)

// DefaultTailBytes bounds the read a statusline render makes of the event log.
const DefaultTailBytes int64 = 2 << 20

// ReadEventsTail reads the events in the last maxBytes of the default event
// log. The first line of a partial window is dropped (it may be cut). A
// missing log is empty, not an error. It never scans the whole file.
func ReadEventsTail(maxBytes int64) ([]Event, error) {
	return ReadEventsTailFrom(defaultEventPath(), maxBytes)
}

// ReadEventsTailFrom is ReadEventsTail for an explicit path.
func ReadEventsTailFrom(path string, maxBytes int64) ([]Event, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if maxBytes <= 0 || size <= maxBytes {
		return parseEvents(f)
	}
	buf := make([]byte, maxBytes)
	if _, err := f.ReadAt(buf, size-maxBytes); err != nil && err != io.EOF {
		return nil, err
	}
	for i, b := range buf {
		if b == '\n' {
			buf = buf[i+1:]
			break
		}
	}
	return parseEvents(bytes.NewReader(buf))
}
