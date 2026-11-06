// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Observability — Loki (logcli), Sentry, Grafana, Datadog
// ════════════════════════════════════════════════════════════════════════════

// Observability family constants. Each family is read-only: it queries logs,
// issues, dashboards or CI results and never creates, updates or deletes.
const (
	FamilyLogCLI     Family = "logcli"
	FamilySentryCLI  Family = "sentry-cli"
	FamilyGrafanaCLI Family = "grafana-cli"
	FamilyDatadogCLI Family = "datadog-ci"
)

// observabilityFamily detects read-only observability CLIs: logcli (Grafana
// Loki), sentry-cli, grafana-cli and datadog-ci. Only query/list/inspect
// subcommands are allowlisted; any send, upload, create, tag or delete
// subcommand returns FamilyNone and is left unwrapped.
func observabilityFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}
	switch argv[0] {
	case "logcli":
		// Loki: query, instant-query, labels and series only read.
		switch argv[1] {
		case "query", "instant-query", "labels", "series":
			return FamilyLogCLI
		}
	case "sentry-cli":
		return sentryCLIFamily(argv[1:])
	case "grafana-cli":
		// Grafana: only the plugin inspection subcommands read state.
		if argv[1] == "plugins" && len(argv) >= 3 {
			switch argv[2] {
			case "ls", "list-remote", "list-versions":
				return FamilyGrafanaCLI
			}
		}
	case "datadog-ci":
		// Datadog: only "version" reads without mutating a CI run. Upload,
		// tag and trace subcommands write and are intentionally excluded.
		if argv[1] == "version" {
			return FamilyDatadogCLI
		}
	}
	return FamilyNone
}

// sentryCLIFamily detects read-only sentry-cli commands, given the words after
// "sentry-cli": "info", and the "list" action of issues/projects/releases.
func sentryCLIFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	if args[0] == "info" {
		return FamilySentryCLI
	}
	switch args[0] {
	case "issues", "projects", "releases":
		if len(args) >= 2 && args[1] == "list" {
			return FamilySentryCLI
		}
	}
	return FamilyNone
}
