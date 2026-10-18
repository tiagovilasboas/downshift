// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// nbopinion's init registers the naive-Bayes hooks that core calls. This
// blank import lives in the external test package: nbopinion imports core, so
// an internal test (package core) cannot import it. The hooks are set before
// any test in this binary runs, including package core tests that need the
// real opinion.
import _ "github.com/tiagovilasboas/downshift/internal/nbopinion"
