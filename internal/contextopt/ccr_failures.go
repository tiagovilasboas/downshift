// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt

// CCRFailuresEnabled resolves whether the raw output of a compacted failed
// command is kept. Default on. DOWNSHIFT_CCR_FAILURES "0" turns it off and "1"
// on; DOWNSHIFT_CCR=0 also turns it off; the environment wins over the file's
// "ccr_failures". A config error leaves the default.
func CCRFailuresEnabled(cfg Config, cfgErr error, env, ccrEnv string) bool {
	switch env {
	case "1":
		return true
	case "0":
		return false
	}
	if ccrEnv == "0" {
		return false
	}
	if cfgErr != nil || cfg.CCRFailures == nil {
		return true
	}
	return *cfg.CCRFailures
}
