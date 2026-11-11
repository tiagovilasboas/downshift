// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt

// BalancedOn reports whether balanced compaction is on: absent or true.
func (c Config) BalancedOn() bool { return c.Balanced == nil || *c.Balanced }

// BalancedEnabled resolves the balanced switch. env "1" turns it on and "0"
// off, and wins over the file. A config that cannot be read means on: the cut
// is only ever made when the raw output is stored, so nothing is lost.
func BalancedEnabled(cfg Config, cfgErr error, env string) bool {
	switch env {
	case "1":
		return true
	case "0":
		return false
	}
	if cfgErr != nil {
		return true
	}
	return cfg.BalancedOn()
}
