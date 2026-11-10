// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package reportview is the styled report screen: a summary first, then one
// scrollable tab per detail section. It is a Bubble Tea model with no program
// inside, and it knows nothing of how the figures are computed: the caller maps
// its own data to Data.
package reportview

// Data is everything the screen draws.
type Data struct {
	Header    Header
	Harnesses []Row
	Tabs      []Tab
}

// Header is the one-line headline. Label is the screen title (default
// "Downshift"); Days is the report window.
type Header struct {
	Label     string
	Days      int
	Commands  int64
	Compacted int64
}

// Row is one harness. Figures are never summed across rows.
type Row struct {
	Harness     string
	TokensSaved int64

	RoutingObserved bool
	RoutedSharePct  float64
	Billable        int64
	// Rewritten is the part of the below-session figure that a Downshift
	// rewrite caused (spawns, billable tokens).
	RewrittenSpawns   int
	RewrittenBillable int64

	PremiumObserved bool
	NonFrontier     int
	TierKnown       int
	PremiumPct      float64

	Reworked  int
	Ended     int
	ReworkPct float64

	// Compactor: measured bytes and counts of the window. TokensSaved is
	// SavedBytes / 4, net of what retrieve read back.
	RawBytes, SavedBytes, RetrievedBytes int64
	CutPct                               float64
	Commands, Compacted                  int64
	HookRuns, DirectRuns                 int64
	// Families is the Compactor by command family, most tokens first.
	Families []Family

	// Spawns is the routed spawns; the verdict counts split those with a session.
	Spawns, ModelSwitched, EffortChanged, Held, HeldGuardrail int
	// Quota is the ready quota text ("5h 42% · week 17%"), "" when unknown.
	Quota string
}

// Family is one command family of the Compactor.
type Family struct {
	Name          string
	AvoidedTokens int64
	CutPct        float64
}

// Tab is one detail section (Savings, Routing, Efficiency, Quota), already
// rendered as plain lines by the caller.
type Tab struct {
	Title string
	Lines []string
}
