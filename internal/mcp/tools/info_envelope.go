// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"errors"
)

var errTooLarge = errors.New("result too large (truncated: true): it exceeds the 64 KiB cap")

// maxResultBytes is the largest result text a tool returns.
const maxResultBytes = 64 << 10

// shrinkFunc returns a smaller version of data, or false when it cannot.
type shrinkFunc func(data any) (any, bool)

// envelopeJSON is the single shape of every information tool result.
type envelopeJSON struct {
	Data              any           `json:"data"`
	Strain            []StrainEntry `json:"strain"`
	StrainUnavailable bool          `json:"strain_unavailable,omitempty"`
	Truncated         bool          `json:"truncated,omitempty"`
}

// envelope reads the strain advice and encodes the result, cutting the data
// until it fits maxResultBytes. A strain failure never fails the call.
func (d Deps) envelope(ctx context.Context, out outcome) (Result, error) {
	e := envelopeJSON{Data: out.data, Strain: []StrainEntry{}}
	if d.Strain == nil {
		e.StrainUnavailable = true
	} else if entries, err := d.Strain.Strain(ctx, out.harnesses); err != nil {
		e.StrainUnavailable = true
	} else if entries != nil {
		e.Strain = entries
	}
	for {
		b, err := json.Marshal(e)
		if err != nil {
			return Result{}, errors.New("result could not be encoded")
		}
		if len(b) <= maxResultBytes {
			return Result{Text: string(b)}, nil
		}
		if out.shrink == nil {
			return Result{}, errTooLarge
		}
		smaller, ok := out.shrink(e.Data)
		if !ok {
			return Result{}, errTooLarge
		}
		e.Data, e.Truncated = smaller, true
	}
}

// halve keeps the first half of a list; false when it cannot shrink further.
func halve[T any](items []T) ([]T, bool) {
	if len(items) <= 1 {
		return items, false
	}
	return items[:len(items)/2], true
}
