// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"strings"
)

// ExtraEnvNamesEnv names the variable that lists extra environment names a
// worker may inherit, comma- or space-separated (for example a Codex provider's
// env_key). Everything not allowed here or below stays out of the worker.
const ExtraEnvNamesEnv = "DOWNSHIFT_WORKER_ENV"

// workerEnvNames are the variables a worker CLI needs to start, find its
// config and reach its provider on macOS, Linux and Windows. Names compare
// case-insensitively, as Windows does.
var workerEnvNames = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
	"TMPDIR": true, "TMP": true, "TEMP": true, "LANG": true, "LANGUAGE": true,
	"TERM": true, "TZ": true, "COLORTERM": true, "NO_COLOR": true, "__CF_USER_TEXT_ENCODING": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true, "ALL_PROXY": true,
	"SSL_CERT_FILE": true, "SSL_CERT_DIR": true, "NODE_EXTRA_CA_CERTS": true, "NODE_OPTIONS": true,
	"USERPROFILE": true, "USERNAME": true, "APPDATA": true, "LOCALAPPDATA": true,
	"PROGRAMDATA": true, "PROGRAMFILES": true, "PROGRAMFILES(X86)": true, "PROGRAMW6432": true,
	"COMMONPROGRAMFILES": true, "SYSTEMROOT": true, "SYSTEMDRIVE": true, "WINDIR": true,
	"COMSPEC": true, "PATHEXT": true, "HOMEDRIVE": true, "HOMEPATH": true, "OS": true,
	"PROCESSOR_ARCHITECTURE": true, "NUMBER_OF_PROCESSORS": true, "COMPUTERNAME": true,
}

// workerEnvPrefixes are whole families a worker may inherit: locale and XDG
// paths, the worker CLIs' own settings and credentials, Downshift's settings,
// and the version managers whose shims launch the CLIs.
var workerEnvPrefixes = []string{
	"LC_", "XDG_", "ANTHROPIC_", "CLAUDE_", "OPENAI_", "CODEX_", "DOWNSHIFT_",
	"ASDF_", "MISE_", "VOLTA_", "FNM_", "NVM_",
}

// providerEnvPrefixes are cloud credentials a worker gets only when the CLI is
// set to use that cloud, keyed by the switch that turns it on.
var providerEnvPrefixes = map[string][]string{
	"CLAUDE_CODE_USE_BEDROCK": {"AWS_"},
	"CLAUDE_CODE_USE_VERTEX":  {"GOOGLE_", "GCLOUD_", "CLOUDSDK_", "CLOUD_ML_", "VERTEX_"},
}

// workerEnv filters the parent environment down to what a worker CLI needs.
// A worker reads files a repository controls, so it should not also hold the
// user's unrelated tokens (GitHub, npm, cloud, database): a prompt-injected
// worker cannot leak what it never received. The entries the spec adds
// (Spec.Env) are appended by the caller and always pass.
func workerEnv(environ []string) []string {
	allowed := map[string]bool{}
	var prefixes []string
	prefixes = append(prefixes, workerEnvPrefixes...)
	for _, kv := range environ {
		name, value, _ := strings.Cut(kv, "=")
		name = strings.ToUpper(name)
		if extra, on := providerEnvPrefixes[name]; on && value != "" {
			prefixes = append(prefixes, extra...)
		}
		if name == ExtraEnvNamesEnv {
			for _, n := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' }) {
				allowed[strings.ToUpper(n)] = true
			}
		}
	}
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		if workerEnvNames[upper] || allowed[upper] || hasAnyPrefix(upper, prefixes) {
			out = append(out, kv)
		}
	}
	return out
}

func hasAnyPrefix(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
