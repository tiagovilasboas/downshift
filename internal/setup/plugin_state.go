// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

// PluginEnabled reads the documented `enabledPlugins` map of a settings file:
// {"enabledPlugins": {"name@marketplace": true}}. Only this file is read;
// project-level settings can also enable a plugin and are not checked here.
func PluginEnabled(raw any, id string) bool {
	root, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	m, ok := root["enabledPlugins"].(map[string]any)
	if !ok {
		return false
	}
	v, ok := m[id].(bool)
	return ok && v
}
