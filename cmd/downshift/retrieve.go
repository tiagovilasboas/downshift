// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/ccr"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// runRetrieve prints the raw output the compactor kept for an id, optionally
// cut to a few lines. It exits 0 on success, 1 when the entry is gone and 2 on
// bad arguments or a malformed id.
func runRetrieve(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("retrieve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var q ccr.Query
	var lines string
	fs.StringVar(&q.Grep, "grep", "", "")
	fs.IntVar(&q.Head, "head", 0, "")
	fs.IntVar(&q.Tail, "tail", 0, "")
	fs.StringVar(&lines, "lines", "", "")
	// Flags may come before or after the id.
	var ids []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return retrieveUsage(stderr, err.Error())
		}
		rest = fs.Args()
		if len(rest) > 0 {
			ids = append(ids, rest[0])
			rest = rest[1:]
		}
	}
	if len(ids) != 1 {
		return retrieveUsage(stderr, "exactly one id is required")
	}
	if q.Head < 0 || q.Tail < 0 {
		return retrieveUsage(stderr, "--head and --tail must not be negative")
	}
	var err error
	if q.From, q.To, err = parseLineRange(lines); err != nil {
		return retrieveUsage(stderr, err.Error())
	}
	return retrieveRun(ids[0], q, stdout, stderr)
}

func retrieveRun(id string, q ccr.Query, stdout, stderr io.Writer) int {
	if !ccr.ValidID(id) {
		return retrieveUsage(stderr, "the id must be 12 lowercase hex characters")
	}
	store, err := ccr.Default()
	if err != nil {
		fmt.Fprintln(stderr, "downshift retrieve: the store is unavailable")
		return 1
	}
	data, err := store.Get(id)
	switch {
	case errors.Is(err, ccr.ErrInvalidID):
		return retrieveUsage(stderr, "the id must be 12 lowercase hex characters")
	case errors.Is(err, ccr.ErrNotFound):
		fmt.Fprintf(stderr, "downshift retrieve: no kept output for %s\n", id)
		return 1
	case errors.Is(err, ccr.ErrExpired):
		fmt.Fprintf(stderr, "downshift retrieve: the kept output for %s has expired\n", id)
		return 1
	case err != nil:
		fmt.Fprintln(stderr, "downshift retrieve: could not read the kept output")
		return 1
	}
	out, err := ccr.Slice(data, q)
	if err != nil {
		return retrieveUsage(stderr, err.Error())
	}
	_, _ = stdout.Write(out)
	execwrap.RecordRetrieval(id, len(out), time.Now())
	return 0
}

func retrieveUsage(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "downshift retrieve: %s\nusage: downshift retrieve <id> [--grep RE] [--head N] [--tail N] [--lines A:B]\n", msg)
	return 2
}

// parseLineRange reads "A:B", "A:" or ":B" (1-based, inclusive). Empty is unset.
func parseLineRange(s string) (from, to int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	a, b, ok := strings.Cut(s, ":")
	if !ok {
		return 0, 0, errors.New("--lines wants A:B")
	}
	if a != "" {
		if from, err = strconv.Atoi(a); err != nil || from < 1 {
			return 0, 0, errors.New("--lines wants positive numbers")
		}
	}
	if b != "" {
		if to, err = strconv.Atoi(b); err != nil || to < 1 {
			return 0, 0, errors.New("--lines wants positive numbers")
		}
	}
	if from > 0 && to > 0 && from > to {
		return 0, 0, errors.New("--lines start is after its end")
	}
	return from, to, nil
}
