// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "os"

// Environment variables read by internal/core.
//
// Every read goes through lookupEnv, the single seam of this package. Tests
// swap it for a map; the target (docs/contrib/layering.md) is configuration
// read once at the edge by the hook runner. This file is the one place to
// check `downshift config` and docs/guides/config.md against.
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
// Only the variables above are read here. DOWNSHIFT_E5_FUSE, DOWNSHIFT_E5_CMD
// and the like are mentioned in e5_*.go comments but not read by core.
const (
	// EnvSessionModels is the operator session allowlist path.
	EnvSessionModels = "DOWNSHIFT_SESSION_MODELS"
	// EnvQuotaMode set to "required" makes quota evidence mandatory.
	EnvQuotaMode = "DOWNSHIFT_QUOTA_MODE"
	// EnvExplicitUpshift set to "1" lets upshift pick explicit_only ids.
	EnvExplicitUpshift = "DOWNSHIFT_EXPLICIT_UPSHIFT"
	// EnvInferSession set to "off" disables the inferred session layer.
	EnvInferSession = "DOWNSHIFT_INFER_SESSION"
	// EnvDiscovery set to "off" disables the discovered-models layer.
	EnvDiscovery = "DOWNSHIFT_DISCOVERY"
	// EnvDiscoveryTTL is a Go duration; "0" keeps a cache entry forever.
	EnvDiscoveryTTL = "DOWNSHIFT_DISCOVERY_TTL"
	// EnvDiscovered overrides the cache path.
	EnvDiscovered = "DOWNSHIFT_DISCOVERED"
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

// lookupEnv is the only reader of the process environment in this package.
var lookupEnv = os.Getenv

func sessionModelsEnv() string                { return lookupEnv(EnvSessionModels) }
func quotaModeRequired() bool                 { return lookupEnv(EnvQuotaMode) == "required" }
func explicitUpshiftEnv() bool                { return lookupEnv(EnvExplicitUpshift) == "1" }
func inferSessionOff() bool                   { return lookupEnv(EnvInferSession) == "off" }
func discoveryOff() bool                      { return lookupEnv(EnvDiscovery) == "off" }
func discoveredEnv() string                   { return lookupEnv(EnvDiscovered) }
func discoveryTTLEnv() string                 { return lookupEnv(EnvDiscoveryTTL) }
func nbUpshiftEnv() bool                      { return lookupEnv(NBUpshiftEnv) == "1" }
func nbDownshiftEnv() bool                    { return lookupEnv(NBDownshiftEnv) == "1" }
func scoreRoutingEnv() bool                   { return lookupEnv(ScoreRoutingEnv) == "1" }
func nativeSessionPathEnv(name string) string { return lookupEnv(name) }
