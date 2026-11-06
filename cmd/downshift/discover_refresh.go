// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/exec"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
)

// Background model discovery trigger (shared hook runner, no harness names).
//
// runHookAdapter calls maybeRefreshDiscovery after the response is written. It
// asks the discovery Refresher, which is the same rule for every harness with a
// listing source: a missing or expired cache entry starts one detached
// `downshift models discover --harness=<h>`. The hook never waits for it and
// never runs a harness CLI itself; the spawn below returns once the process
// starts. The next spawn reads the cache the round wrote.
var discoverRefreshSpawn = spawnDiscoverRefresh

// maybeRefreshDiscovery is fail-open: it never panics and never prints.
func maybeRefreshDiscovery(harness string) {
	defer func() { _ = recover() }()
	cfg := core.ConfigFromEnv(hookGetenv)
	if cfg.DiscoveryOff || cfg.DiscoveryTTLInvalid {
		return
	}
	path, err := discovery.Path()
	if err != nil {
		return
	}
	r := discovery.Refresher{
		CachePath: path,
		Sources:   discovery.Defaults(),
		TTL:       cfg.DiscoveryTTL,
		Spawn:     discoverRefreshSpawn,
	}
	_, _ = r.Trigger(harness)
}

// spawnDiscoverRefresh runs this binary detached, with stdio on the null device.
func spawnDiscoverRefresh(harness string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "models", "discover", "--harness="+harness)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
