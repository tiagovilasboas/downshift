// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"os"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/session"
)

// Environment variables read by internal/core.
//
// Configuration is read once at the edge: the hook runner calls
// SetConfig(ConfigFromEnv(os.Getenv)) at the start of every invocation and
// every accessor below reads that snapshot. Outside a hook (offline commands,
// tests) no config is set and the accessors read the live environment
// through lookupEnv, so their values equal ConfigFromEnv(os.Getenv) at any
// moment. This file is the one place to check `downshift config` and
// docs/guides/config.md against.
//
//	Variable                    Default      Effect
//	DOWNSHIFT_SESSION_MODELS    unset        Path of the operator session
//	                                         allowlist (session-models.json).
//	                                         Unset or empty: the default path
//	                                         from DefaultSessionModelsPath. Also
//	                                         the file read for the
//	                                         "explicit_upshift" flag.
//	DOWNSHIFT_QUOTA_MODE        unset        "required" makes a usage snapshot
//	                                         mandatory (the quota gate stays
//	                                         closed without evidence and
//	                                         abandoned snapshots still hold).
//	                                         Anything else is legacy mode.
//	DOWNSHIFT_EXPLICIT_UPSHIFT  unset        "1" lets upshift select catalog
//	                                         explicit_only ids. The same flag
//	                                         in session-models.json is enough.
//	DOWNSHIFT_INFER_SESSION     on           "off" disables the inferred
//	                                         session layer.
//	DOWNSHIFT_DISCOVERY         on           "off" disables the
//	                                         discovered-models layer.
//	DOWNSHIFT_DISCOVERED        state dir    Overrides the path of
//	                                         discovered.json.
//	DOWNSHIFT_DISCOVERY_TTL     24h          Go duration for a discovery cache
//	                                         entry; "0" keeps it forever. An
//	                                         invalid or negative value makes
//	                                         the entry an unknown session.
//	DOWNSHIFT_NB_UPSHIFT        off          "1" applies the naive-Bayes
//	                                         upshift. Off: the opinion is
//	                                         computed and recorded (shadow)
//	                                         but the tier is unchanged.
//	DOWNSHIFT_NB_DOWNSHIFT      off          "1" applies the naive-Bayes
//	                                         TRIVIAL downshift. Off: shadow
//	                                         only. Guardrail R1 still holds
//	                                         an unconfident downshift.
//	DOWNSHIFT_SCORE_ROUTING     off          "1" lets the public-score
//	                                         capability pick change the model.
//	                                         Off: shadow only.
//	<HarnessCapabilities.       unset        Per-harness path of a native
//	 NativeSessionEnv>                       session export (Cursor:
//	                                         DOWNSHIFT_CURSOR_NATIVE_FILE).
//	                                         Name comes from the capability
//	                                         table, never a branch.
//
// Only the variables above are read here.
const (
	// EnvSessionModels is the operator session allowlist path.
	EnvSessionModels = session.EnvSessionModels
	// EnvQuotaMode set to "required" makes quota evidence mandatory.
	EnvQuotaMode = session.EnvQuotaMode
	// EnvExplicitUpshift set to "1" lets upshift pick explicit_only ids.
	EnvExplicitUpshift = session.EnvExplicitUpshift
	// EnvInferSession set to "off" disables the inferred session layer.
	EnvInferSession = "DOWNSHIFT_INFER_SESSION"
	// EnvDiscovery set to "off" disables the discovered-models layer.
	EnvDiscovery = session.EnvDiscovery
	// EnvDiscoveryTTL is a Go duration; "0" keeps a cache entry forever.
	EnvDiscoveryTTL = session.EnvDiscoveryTTL
	// EnvDiscovered overrides the cache path.
	EnvDiscovered = session.EnvDiscovered
	// NBUpshiftEnv turns the naive-Bayes upshift on. Default OFF: the opinion
	// is still computed and recorded (shadow mode) but the routing tier is
	// unchanged.
	NBUpshiftEnv = "DOWNSHIFT_NB_UPSHIFT"
	// NBDownshiftEnv turns the naive-Bayes TRIVIAL downshift on. Default OFF:
	// the opinion is still computed and recorded (shadow mode) but routing is
	// unchanged.
	NBDownshiftEnv = "DOWNSHIFT_NB_DOWNSHIFT"
	// ScoreRoutingEnv makes the capability pick live. Default off (shadow only).
	ScoreRoutingEnv = "DOWNSHIFT_SCORE_ROUTING"
)

// defaultDiscoveryTTL is the discovery cache TTL when DOWNSHIFT_DISCOVERY_TTL
// is unset. The loader that honors it lives in internal/session.
const defaultDiscoveryTTL = session.DefaultDiscoveryTTL

// Config is every setting core reads from the environment, typed. The zero
// value is "nothing set". Build one with ConfigFromEnv; install it with
// SetConfig.
type Config struct {
	// SessionModelsPath is DOWNSHIFT_SESSION_MODELS; "" means the default path.
	SessionModelsPath string
	// QuotaRequired is DOWNSHIFT_QUOTA_MODE == "required".
	QuotaRequired bool
	// ExplicitUpshift is DOWNSHIFT_EXPLICIT_UPSHIFT == "1".
	ExplicitUpshift bool
	// InferSessionOff is DOWNSHIFT_INFER_SESSION == "off".
	InferSessionOff bool
	// DiscoveryOff is DOWNSHIFT_DISCOVERY == "off".
	DiscoveryOff bool
	// DiscoveredPath is DOWNSHIFT_DISCOVERED; "" means the state dir.
	DiscoveredPath string
	// DiscoveryTTL is DOWNSHIFT_DISCOVERY_TTL parsed as a Go duration, or
	// session.DefaultDiscoveryTTL when unset; 0 keeps a cache entry forever.
	DiscoveryTTL time.Duration
	// DiscoveryTTLInvalid is set when DOWNSHIFT_DISCOVERY_TTL is present but
	// not a non-negative Go duration. A discovered entry is then an unknown
	// session (same behaviour as before the value was typed).
	DiscoveryTTLInvalid bool
	// NBUpshift is DOWNSHIFT_NB_UPSHIFT == "1".
	NBUpshift bool
	// NBDownshift is DOWNSHIFT_NB_DOWNSHIFT == "1".
	NBDownshift bool
	// ScoreRouting is DOWNSHIFT_SCORE_ROUTING == "1".
	ScoreRouting bool
	// NativeSessionPaths holds the value of every HarnessCapabilities.
	// NativeSessionEnv variable, keyed by variable name. The names come from
	// the capability table, never from a branch.
	NativeSessionPaths map[string]string
}

// ConfigFromEnv reads every variable above through getenv, once. Matching is
// exact and case-sensitive: only the documented literal values switch a flag.
func ConfigFromEnv(getenv func(string) string) Config {
	sc := session.ConfigFromEnv(getenv)
	c := Config{
		SessionModelsPath:   sc.SessionModelsPath,
		QuotaRequired:       sc.QuotaRequired,
		ExplicitUpshift:     sc.ExplicitUpshift,
		InferSessionOff:     getenv(EnvInferSession) == "off",
		DiscoveryOff:        sc.DiscoveryOff,
		DiscoveredPath:      sc.DiscoveredPath,
		DiscoveryTTL:        sc.DiscoveryTTL,
		DiscoveryTTLInvalid: sc.DiscoveryTTLInvalid,
		NBUpshift:           getenv(NBUpshiftEnv) == "1",
		NBDownshift:         getenv(NBDownshiftEnv) == "1",
		ScoreRouting:        getenv(ScoreRoutingEnv) == "1",
	}
	for _, n := range AllHarnessCaps() {
		name := n.Caps.NativeSessionEnv
		if name == "" {
			continue
		}
		if c.NativeSessionPaths == nil {
			c.NativeSessionPaths = map[string]string{}
		}
		c.NativeSessionPaths[name] = getenv(name)
	}
	return c
}

var (
	configMu sync.RWMutex
	// activeConfig is the snapshot every accessor reads. nil means no config
	// was set: the accessors read the live environment through lookupEnv.
	activeConfig *Config
)

// SetConfig installs c as the configuration every accessor reads and returns
// a function that restores the previous state. The hook runner calls it once
// per invocation; tests call it to pin a configuration without touching the
// process environment.
func SetConfig(c Config) (restore func()) {
	configMu.Lock()
	prev := activeConfig
	activeConfig = &c
	configMu.Unlock()
	restoreSession := session.SetConfig(session.Config{
		SessionModelsPath:   c.SessionModelsPath,
		QuotaRequired:       c.QuotaRequired,
		ExplicitUpshift:     c.ExplicitUpshift,
		DiscoveryOff:        c.DiscoveryOff,
		DiscoveredPath:      c.DiscoveredPath,
		DiscoveryTTL:        c.DiscoveryTTL,
		DiscoveryTTLInvalid: c.DiscoveryTTLInvalid,
	})
	return func() {
		restoreSession()
		configMu.Lock()
		activeConfig = prev
		configMu.Unlock()
	}
}

// config returns the active snapshot, or a fresh read of the live
// environment when none is set.
func config() Config {
	configMu.RLock()
	c := activeConfig
	configMu.RUnlock()
	if c != nil {
		return *c
	}
	return ConfigFromEnv(lookupEnv)
}

// lookupEnv is the only reader of the process environment in this package.
// It is consulted only when no Config is set.
var lookupEnv = os.Getenv

func sessionModelsEnv() string { return config().SessionModelsPath }
func quotaModeRequired() bool  { return config().QuotaRequired }
func explicitUpshiftEnv() bool { return config().ExplicitUpshift }
func inferSessionOff() bool    { return config().InferSessionOff }
func discoveryOff() bool       { return config().DiscoveryOff }
func discoveredEnv() string    { return config().DiscoveredPath }
func nbUpshiftEnv() bool       { return config().NBUpshift }
func nbDownshiftEnv() bool     { return config().NBDownshift }
func scoreRoutingEnv() bool    { return config().ScoreRouting }

// nativeSessionPathEnv returns the path held by the named native-export
// variable. A pinned Config carries every name the capability table declares;
// without one the live environment is read, so a harness declared outside
// AllHarnessCaps (tests) still works.
func nativeSessionPathEnv(name string) string {
	if name == "" {
		return ""
	}
	configMu.RLock()
	c := activeConfig
	configMu.RUnlock()
	if c != nil {
		return c.NativeSessionPaths[name]
	}
	return lookupEnv(name)
}

// discoveryTTL returns the discovery cache TTL and whether the configured
// value was invalid (an unknown session, see LoadDiscoveredSession).
func discoveryTTL() (ttl time.Duration, invalid bool) {
	c := config()
	return c.DiscoveryTTL, c.DiscoveryTTLInvalid
}
