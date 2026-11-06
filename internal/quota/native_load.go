// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// LoadNative reads an opt-in provider export before the canonical quota cache.
// Hooks remain offline: this only reads a bounded regular file written
// by a native bridge, and never starts a provider CLI or performs auth.
// A configured but invalid file is reported as present so callers fail closed
// instead of silently falling back to an older or handwritten source.
func LoadNative(harness string) (*Snapshot, bool) {
	path, source, ok := nativePath(harness)
	if !ok {
		return nil, false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, true
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, true
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(b) > 1<<20 {
		return nil, true
	}
	if source == "cursor-usage" && !hasObservedAt(b) {
		return nil, true
	}
	s, err := ParseNative(source, b, time.Now().UTC(), 5*time.Minute)
	if err != nil {
		return nil, true
	}
	return &s, true
}

func hasObservedAt(data []byte) bool {
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return false
	}
	for _, key := range []string{"observed_at", "observedAt"} {
		if value, ok := raw[key]; ok {
			var at time.Time
			return json.Unmarshal(value, &at) == nil && !at.IsZero()
		}
	}
	return false
}

// nativeSources maps a harness to its native export env var and parser source.
// The facts are owned by core.HarnessCapabilities. quota's init publishes them
// through core.RegisterNativeQuotaHook, so there is no second copy here.
var (
	nativeMu      sync.RWMutex
	nativeSources = map[string]struct{ env, source string }{}
)

// RegisterNativeSource declares a harness's native export. Empty values are
// ignored.
func RegisterNativeSource(harness, env, source string) {
	if harness == "" || env == "" || source == "" {
		return
	}
	nativeMu.Lock()
	nativeSources[harness] = struct{ env, source string }{env, source}
	nativeMu.Unlock()
}

// NativeSource reports the env var and parser source of a harness's export.
func NativeSource(harness string) (env, source string, ok bool) {
	nativeMu.RLock()
	n, ok := nativeSources[harness]
	nativeMu.RUnlock()
	return n.env, n.source, ok
}

func nativePath(harness string) (path, source string, ok bool) {
	env, source, found := NativeSource(harness)
	if !found {
		return "", "", false
	}
	path = os.Getenv(env)
	return path, source, path != ""
}
