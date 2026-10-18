<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# S2: setup and once-a-day trigger

- Shared hook runner (`runHookAdapter` in cmd/downshift): after the response is
  written and flushed, `scoreindex.MaybeRefreshDetached()`: if `DOWNSHIFT_SCORES`
  is not off and `scores.json` is missing or older than 24 h (one `stat`), start a
  detached process `downshift models scores --refresh --quiet` (own session,
  stdin/stdout/stderr to /dev/null, no wait), guarded by a lock file with a
  10-minute stale timeout so parallel hooks start at most one. The hook never
  waits, never prints, never fails because of it. Put the spawn behind a
  package-level func variable so tests inject a fake.
- Setup: `install.sh` runs `downshift models scores --refresh --quiet` best effort
  (failure never fails the install); `downshift doctor` prints the score-cache
  status and starts a refresh when stale.
- Tests through `runHookAdapter`: stale cache spawns exactly once; fresh cache,
  `DOWNSHIFT_SCORES=off`, and spawn error all leave the hook output unchanged;
  hook latency not affected (fake spawn). Works for every harness registered in
  the harness registry, no harness name branches.
