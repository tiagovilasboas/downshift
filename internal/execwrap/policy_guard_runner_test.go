// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

// A guarded program (a recursive rm, mysql, a Redis write) must not reach a
// wrap through a runner that starts another program, nor ride along as a raw
// segment of a rewritten chain.

const guardRunnerExe = "/opt/ds/downshift"

func TestGuardedCommandNeverRewrittenThroughARunner(t *testing.T) {
	for _, cmd := range []string{
		"direnv exec . rm -rf build",
		"direnv exec . redis-cli flushall",
		"direnv exec . mysql -e drop",
		"direnv exec . docker exec app rm -rf data",
		"lerna exec rm -rf build",
		"lerna exec -- rm -rf build",
		"lerna exec --scope web -- rm -rf build",
		"lerna exec --scope web rm -rf build",
		"adb shell rm -rf /sdcard",
		"uv run rm -rf build",
		"uv run /bin/rm -rf build",
		"poetry run rm -rf build",
		"pdm run redis-cli flushall",
	} {
		if d := Decide(cmd, guardRunnerExe); d.Rewrite {
			t.Errorf("%q rewritten to %q, want it left alone", cmd, d.Command)
		}
	}
}

func TestGuardedCommandNeverRewrittenInAChain(t *testing.T) {
	for _, cmd := range []string{
		"git status && rm -rf build",
		"git status ; redis-cli flushall",
		"git status && valkey-cli flushall",
		"go test ./... && mysql -e 'drop table shop'",
		"go test ./... && mysql -e drop",
		"git status && FOO=1 rm -rf build",
		"git status && sudo rm -rf build",
		"git status && /bin/rm -rf build",
		"rm -rf build && git status",
	} {
		if d := Decide(cmd, guardRunnerExe); d.Rewrite {
			t.Errorf("%q rewritten to %q, want it left alone", cmd, d.Command)
		}
	}
}

func TestLegitRunnersAndChainsStillRewrite(t *testing.T) {
	for _, cmd := range []string{
		"direnv exec . go test ./...",
		"direnv status",
		"lerna exec -- npm test",
		"lerna exec --scope web -- npm test",
		"lerna run build",
		"uv run pytest",
		"poetry run pytest",
		"git status && go test ./...",
		"cd app && go build ./...",
		"git status && rm build.log", // a plain rm is a family of its own
	} {
		if d := Decide(cmd, guardRunnerExe); !d.Rewrite {
			t.Errorf("%q not rewritten (reason %q), want a wrap", cmd, d.Reason)
		}
	}
}
