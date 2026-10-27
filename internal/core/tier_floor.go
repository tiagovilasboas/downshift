// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// TierFloorHook lets an optional layer raise a decision to an operator-defined
// minimum tier. It is nil unless that layer is linked in, and it may only
// raise: implementations call Decision.RaiseTier. It runs inside Route, so
// every harness adapter inherits it without any adapter code.
var TierFloorHook func(d *Decision, currentModelID string, r Resolver)
