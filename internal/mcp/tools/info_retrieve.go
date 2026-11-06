// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/ccr"
)

const retrieveSchema = `{"type":"object","properties":{` +
	`"id":{"type":"string","description":"The id from the compactor marker: 12 lowercase hex characters."},` +
	`"grep":{"type":"string","description":"Keep only lines matching this RE2 pattern."},` +
	`"head":{"type":"integer","minimum":0,"description":"Keep the first N lines."},` +
	`"tail":{"type":"integer","minimum":0,"description":"Keep the last N lines."},` +
	`"lines":{"type":"string","description":"Keep lines A:B (1-based, inclusive)."}},` +
	`"required":["id"],"additionalProperties":false}`

type retrieveArgs struct {
	ID    string `json:"id"`
	Grep  string `json:"grep"`
	Head  int    `json:"head"`
	Tail  int    `json:"tail"`
	Lines string `json:"lines"`
}

// retrieveTool returns the raw output the compactor kept for an id. It returns
// the lines as plain text, not the status envelope, and reports every failure
// as an error result the model can read.
func retrieveTool() Tool {
	return Tool{
		Name:        "retrieve",
		Description: "Returns the full output the Downshift compactor kept when it cut a command's output. The cut output ends with a marker naming an id; pass that id here, and narrow with grep, head, tail or lines (A:B) to fetch only the lines you need. Kept output expires after 24 hours.",
		InputSchema: json.RawMessage(retrieveSchema),
		Title:       "Kept command output", ReadOnly: true, Idempotent: true,
		Handler: func(_ context.Context, raw json.RawMessage) (Result, error) {
			return runRetrieve(raw), nil
		},
	}
}

func runRetrieve(raw json.RawMessage) Result {
	var a retrieveArgs
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := dec.Decode(&a); err != nil {
			return failure("arguments must be an object with id, grep, head, tail and lines")
		}
	}
	if !ccr.ValidID(a.ID) {
		return failure("id must be 12 lowercase hex characters")
	}
	if a.Head < 0 || a.Tail < 0 {
		return failure("head and tail must not be negative")
	}
	from, to, err := parseLines(a.Lines)
	if err != nil {
		return failure(err.Error())
	}
	store, err := ccr.Default()
	if err != nil {
		return failure("the store is unavailable")
	}
	data, err := store.Get(a.ID)
	switch {
	case errors.Is(err, ccr.ErrNotFound):
		return failure("no kept output for this id")
	case errors.Is(err, ccr.ErrExpired):
		return failure("the kept output for this id has expired")
	case err != nil:
		return failure("could not read the kept output")
	}
	out, err := ccr.Slice(data, ccr.Query{Grep: a.Grep, Head: a.Head, Tail: a.Tail, From: from, To: to})
	if err != nil {
		return failure("invalid grep pattern or range")
	}
	if len(out) > maxResultBytes {
		out = append(out[:maxResultBytes:maxResultBytes],
			fmt.Sprintf("\n[downshift: cut at %d bytes; narrow with grep, head, tail or lines]\n", maxResultBytes)...)
	}
	return Result{Text: string(out)}
}

func failure(msg string) Result { return Result{Text: msg, IsError: true} }

// parseLines reads "A:B", "A:" or ":B"; empty means no range.
func parseLines(s string) (from, to int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	a, b, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, errors.New("lines must look like A:B")
	}
	for i, part := range []string{a, b} {
		if part == "" {
			continue
		}
		n, perr := strconv.Atoi(part)
		if perr != nil || n < 1 {
			return 0, 0, errors.New("lines must hold positive numbers")
		}
		if i == 0 {
			from = n
		} else {
			to = n
		}
	}
	if from > 0 && to > 0 && from > to {
		return 0, 0, errors.New("lines start is after its end")
	}
	return from, to, nil
}
