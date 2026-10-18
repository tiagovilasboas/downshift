// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Quota's init registers the file and native hooks that core calls. This
// blank import lives in the external test package: quota imports core, so an
// internal test (package core) cannot import it. The hooks are set before any
// test in this binary runs, including package core tests that read quota files.
import _ "github.com/tiagovilasboas/downshift/internal/quota"
