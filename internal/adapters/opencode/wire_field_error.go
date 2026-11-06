// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"encoding/json"
	"errors"
)

// fieldError is a decode error that also names the payload field. The
// message is the underlying error; the field is a JSON path, never a value.
type fieldError struct {
	err   error
	field string
}

func (e *fieldError) Error() string        { return e.err.Error() }
func (e *fieldError) Unwrap() error        { return e.err }
func (e *fieldError) InvalidField() string { return e.field }

// decodeField is the JSON path of the field with the wrong type, or "" when
// the payload is not valid JSON at all.
func decodeField(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return typeErr.Field
	}
	return ""
}
