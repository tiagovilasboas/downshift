// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "os"

// A git hook (pre-commit, pre-push) runs these tests with GIT_DIR and friends
// pointing at the developer's real repository. A test that runs git in a temp
// dir would then write there (git init set core.bare=true and git config set a
// test identity in the shared .git/config on 2026-10-24). Drop them before any
// test runs, so git finds its repository from the working directory.
func init() {
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_PREFIX"} {
		_ = os.Unsetenv(k)
	}
}
