// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// ConfigError refuses a policy file or a request. Source is the file path, or
// empty for a request. Field names the setting, for example
// routing.rules[2].profile. Reason is set where a routing note can carry it
// (an unknown profile, a profile that is overridden, a foreign harness).
type ConfigError struct {
	Source  string
	Field   string
	Problem string
	Reason  contract.Reason
}

// Error formats the refusal with its source and field.
func (e *ConfigError) Error() string {
	if e.Source == "" {
		return fmt.Sprintf("policy: %s: %s", e.Field, e.Problem)
	}
	return fmt.Sprintf("policy: %s: %s: %s", e.Source, e.Field, e.Problem)
}

func refuse(source, field, format string, args ...any) error {
	return &ConfigError{Source: source, Field: field, Problem: fmt.Sprintf(format, args...)}
}
