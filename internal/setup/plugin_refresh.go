// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"strings"
)

// refreshPlugin updates an already-installed plugin to the marketplace's
// latest commit. Claude Code pins an install to the commit it fetched and does
// not update a third-party marketplace by itself, so without this an upgrade
// of the binary leaves the plugin's agents, skills and hooks behind. A failure
// is a note, never an error: the installed plugin keeps working.
func refreshPlugin(o Options, t Target) []string {
	marketplace := pluginMarketplaceName(t.Plugin)
	updateMarket := "claude plugin marketplace update " + marketplace
	updatePlugin := "claude plugin update " + t.Plugin
	manual := "run `" + updateMarket + "` and `" + updatePlugin + "`, then open a new session"
	if o.DryRun {
		return []string{"would run `" + updateMarket + "` and `" + updatePlugin + "` (the plugin's agents, skills and hooks follow the latest release)"}
	}
	claude, err := o.Exec.LookPath("claude")
	if err != nil {
		return []string{"`claude` is not on PATH, so the installed " + t.Plugin + " plugin was not updated; " + manual}
	}
	if out, err := o.Exec.Run(claude, "plugin", "marketplace", "update", marketplace); err != nil {
		return []string{"`" + updateMarket + "` failed (" + oneLine(out, err) + "); " + manual}
	}
	if out, err := o.Exec.Run(claude, "plugin", "update", t.Plugin); err != nil && !pluginAlreadyDone(out) {
		return []string{"`" + updatePlugin + "` failed (" + oneLine(out, err) + "); " + manual}
	}
	return []string{"updated the " + t.Plugin + " plugin to the latest release: open a new session to load its agents and skills"}
}

// pluginMarketplaceName is the marketplace part of a plugin id ("name@marketplace").
func pluginMarketplaceName(plugin string) string {
	if _, market, ok := strings.Cut(plugin, "@"); ok {
		return market
	}
	return plugin
}
