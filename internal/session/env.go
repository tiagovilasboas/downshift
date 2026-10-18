// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"os"
	"sync"
	"time"
)

// Environment variables the session list reads. core.ConfigFromEnv copies
// these fields into the hook snapshot and SetConfig forwards that snapshot
// here, so a pinned configuration and the live environment stay one source.
const (
	// EnvSessionModels is the operator session allowlist path.
	EnvSessionModels = "DOWNSHIFT_SESSION_MODELS"
	// EnvQuotaMode set to "required" makes quota evidence mandatory.
	EnvQuotaMode = "DOWNSHIFT_QUOTA_MODE"
	// EnvExplicitUpshift set to "1" lets upshift pick explicit_only ids.
	EnvExplicitUpshift = "DOWNSHIFT_EXPLICIT_UPSHIFT"
	// EnvDiscovery set to "off" disables the discovered-models layer.
	EnvDiscovery = "DOWNSHIFT_DISCOVERY"
	// EnvDiscoveryTTL is a Go duration; "0" keeps a cache entry forever.
	EnvDiscoveryTTL = "DOWNSHIFT_DISCOVERY_TTL"
	// EnvDiscovered overrides the cache path.
	EnvDiscovered = "DOWNSHIFT_DISCOVERED"

	// DefaultDiscoveryTTL is the discovery cache TTL when EnvDiscoveryTTL is unset.
	DefaultDiscoveryTTL = 24 * time.Hour
)

// Config is the session-list slice of the process configuration.
type Config struct {
	// SessionModelsPath is DOWNSHIFT_SESSION_MODELS; "" means the default path.
	SessionModelsPath string
	// QuotaRequired is DOWNSHIFT_QUOTA_MODE == "required".
	QuotaRequired bool
	// ExplicitUpshift is DOWNSHIFT_EXPLICIT_UPSHIFT == "1".
	ExplicitUpshift bool
	// DiscoveryOff is DOWNSHIFT_DISCOVERY == "off".
	DiscoveryOff bool
	// DiscoveredPath is DOWNSHIFT_DISCOVERED; "" means the state dir.
	DiscoveredPath string
	// DiscoveryTTL is DOWNSHIFT_DISCOVERY_TTL parsed as a Go duration, or
	// DefaultDiscoveryTTL when unset; 0 keeps a cache entry forever.
	DiscoveryTTL time.Duration
	// DiscoveryTTLInvalid is set when DOWNSHIFT_DISCOVERY_TTL is present but
	// not a non-negative Go duration. A discovered entry is then an unknown
	// session.
	DiscoveryTTLInvalid bool
}

// ConfigFromEnv reads the session-list variables through getenv, once.
// Matching is exact and case-sensitive: only the documented literal values
// switch a flag. A nil getenv reads the process environment.
func ConfigFromEnv(getenv func(string) string) Config {
	if getenv == nil {
		getenv = os.Getenv
	}
	c := Config{
		SessionModelsPath: getenv(EnvSessionModels),
		QuotaRequired:     getenv(EnvQuotaMode) == "required",
		ExplicitUpshift:   getenv(EnvExplicitUpshift) == "1",
		DiscoveryOff:      getenv(EnvDiscovery) == "off",
		DiscoveredPath:    getenv(EnvDiscovered),
		DiscoveryTTL:      DefaultDiscoveryTTL,
	}
	if v := getenv(EnvDiscoveryTTL); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			c.DiscoveryTTLInvalid = true
		} else {
			c.DiscoveryTTL = d
		}
	}
	return c
}

var (
	mu sync.RWMutex
	// active is the snapshot every accessor reads. nil means no config was
	// set: the accessors read through lookupEnv.
	active *Config
	// lookupEnv is the live environment reader used when no Config is set.
	// core installs a closure over its own seam so tests that replace that
	// seam stay visible here.
	lookupEnv = os.Getenv
)

// SetLookup installs the environment reader used when no Config is set.
// A nil fn restores os.Getenv. core points this at its own lookup seam.
func SetLookup(fn func(string) string) {
	mu.Lock()
	defer mu.Unlock()
	if fn == nil {
		lookupEnv = os.Getenv
		return
	}
	lookupEnv = fn
}

// SetConfig installs c as the configuration every accessor reads and returns
// a function that restores the previous state. core.SetConfig forwards the
// session-list fields so the hook snapshot and these loaders cannot diverge.
func SetConfig(c Config) (restore func()) {
	mu.Lock()
	prev := active
	active = &c
	mu.Unlock()
	return func() {
		mu.Lock()
		active = prev
		mu.Unlock()
	}
}

func config() Config {
	mu.RLock()
	c := active
	fn := lookupEnv
	mu.RUnlock()
	if c != nil {
		return *c
	}
	return ConfigFromEnv(fn)
}

func sessionModelsEnv() string { return config().SessionModelsPath }
func quotaModeRequired() bool  { return config().QuotaRequired }
func explicitUpshiftEnv() bool { return config().ExplicitUpshift }
func discoveryOff() bool       { return config().DiscoveryOff }
func discoveredEnv() string    { return config().DiscoveredPath }

func discoveryTTL() (ttl time.Duration, invalid bool) {
	c := config()
	return c.DiscoveryTTL, c.DiscoveryTTLInvalid
}
