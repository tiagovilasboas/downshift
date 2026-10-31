// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// childEndModel is the model a child_ended observation records: the model the
// child actually ran on. The adapter reads it from the child's own transcript
// (the first assistant message's model, normalized to a catalog id) when one is
// available; the model the harness said it resolved at launch is the fallback.
// It stays "unknown" only when neither is known, bounded like all untrusted
// hook metadata before durable logging.
func childEndModel(ce hookport.ChildEnd) string {
	served := ce.ServedModel
	if served == "" {
		served = ce.ResolvedModel
	}
	return telemetry.ModelOrUnknown(served)
}
