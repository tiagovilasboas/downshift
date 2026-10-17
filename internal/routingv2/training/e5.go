// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package training

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// E5ShadowSummary aggregates the prompt-free `e5_shadow` objects that
// decision events carry when DOWNSHIFT_E5_SHADOW=1. It reads only the closed
// field set of telemetry.E5ShadowRecord (labels, flags, latency, error code);
// no prompt, path or helper output exists in the input, so none can appear
// here. Rates use Scored (helper answered) as the denominator: an observation
// that failed cannot have a nearest label or a lift.
type E5ShadowSummary struct {
	// Observed is the number of decision events carrying e5_shadow.
	Observed int `json:"observed"`
	// Errors is the number of observations whose helper failed; ErrorCodes
	// is their histogram by closed error code.
	Errors     int            `json:"errors"`
	ErrorCodes map[string]int `json:"error_codes,omitempty"`
	// Scored is Observed minus Errors: observations with a nearest label.
	Scored int `json:"scored"`
	// WouldLift counts scored observations where E5 would have raised the
	// complexity; WouldLiftRate is WouldLift / Scored.
	WouldLift     int     `json:"would_lift"`
	WouldLiftRate float64 `json:"would_lift_rate"`
	// ByRegexLabel splits the lift rate by the regex classifier's own label,
	// ordered TRIVIAL, SIMPLE, MEDIUM, COMPLEX, then any other label by name.
	ByRegexLabel []E5RegexLabelStats `json:"by_regex_label,omitempty"`
	// NearestLabels is the histogram of nearest centroid labels over Scored.
	NearestLabels map[string]int `json:"nearest_labels,omitempty"`
	// Abstain counts scored observations whose nearest label is a reserved
	// one (ABSTAIN or FOREIGN); AbstainShare is Abstain / Scored.
	Abstain      int     `json:"abstain"`
	AbstainShare float64 `json:"abstain_share"`
	// RiskDecisions counts scored observations on a decision whose reason
	// codes include RISK_CATEGORY or RISK_FLOOR.
	RiskDecisions int `json:"risk_decisions"`
	// FalseDowngradeProxy counts RiskDecisions where would_lift is false and
	// the nearest label ranks strictly below regex_label on the
	// TRIVIAL < SIMPLE < MEDIUM < COMPLEX scale. It is a proxy, not a measured
	// error: the shadow never changed the route, no reviewer labelled the
	// task, and a risk floor would have held the tier anyway. It flags the
	// pattern that would be dangerous if the fuse were on and the floor were
	// not: E5 sees less than the regex on a prompt the risk words caught.
	// Observations whose nearest label is reserved (ABSTAIN, FOREIGN) or
	// unknown do not rank and are never counted.
	FalseDowngradeProxy int `json:"false_downgrade_proxy"`
	// Latency is over Scored observations (the helper answered), so a
	// missing_cmd at 0 ms does not pull the median down and a timeout does
	// not stand in for engine latency; timeouts show in ErrorCodes.
	Latency LatencyMs `json:"latency_ms"`
}

// E5RegexLabelStats is the lift rate for one regex label.
type E5RegexLabelStats struct {
	RegexLabel    string  `json:"regex_label"`
	Scored        int     `json:"scored"`
	WouldLift     int     `json:"would_lift"`
	WouldLiftRate float64 `json:"would_lift_rate"`
}

// E5FuseSummary aggregates the `e5_fuse` objects written when
// DOWNSHIFT_E5_FUSE=1. Same prompt-free input and same denominators as
// E5ShadowSummary.
type E5FuseSummary struct {
	Observed int `json:"observed"`
	// Actions is the histogram of none / abstain / raise over answered
	// observations (an error leaves action empty and is not counted here).
	Actions    map[string]int `json:"actions,omitempty"`
	Errors     int            `json:"errors"`
	ErrorCodes map[string]int `json:"error_codes,omitempty"`
	// Latency is over answered observations.
	Latency LatencyMs `json:"latency_ms"`
}

// LatencyMs holds nearest-rank percentiles of latency_ms samples: the
// p-th percentile is the value at index ceil(p*n)-1 of the sorted sample.
// With one sample every field is that sample; with none every field is 0.
type LatencyMs struct {
	Samples int     `json:"samples"`
	P50     float64 `json:"p50"`
	P95     float64 `json:"p95"`
	Max     float64 `json:"max"`
}

// SummarizeE5 reads the E5 objects off decision events (never cost-only or
// error records). Either result is nil when no event carries that object, so
// a log written without the flags adds no key to the report.
func SummarizeE5(events []telemetry.Event) (*E5ShadowSummary, *E5FuseSummary) {
	var shadow *E5ShadowSummary
	var fuse *E5FuseSummary
	var shadowLat, fuseLat []float64
	byRegex := map[string]*E5RegexLabelStats{}
	for _, ev := range events {
		if ev.Outcome == "error" || telemetry.IsCostOnlyOutcome(ev.Outcome) {
			continue
		}
		if o := ev.E5Shadow; o != nil {
			if shadow == nil {
				shadow = &E5ShadowSummary{ErrorCodes: map[string]int{}, NearestLabels: map[string]int{}}
			}
			shadow.Observed++
			if o.ErrorCode != "" {
				shadow.Errors++
				shadow.ErrorCodes[o.ErrorCode]++
			} else {
				shadow.Scored++
				shadowLat = append(shadowLat, o.LatencyMs)
				shadow.NearestLabels[o.NearestLabel]++
				r := byRegex[o.RegexLabel]
				if r == nil {
					r = &E5RegexLabelStats{RegexLabel: o.RegexLabel}
					byRegex[o.RegexLabel] = r
				}
				r.Scored++
				if o.WouldLift {
					shadow.WouldLift++
					r.WouldLift++
				}
				if e5shadow.LanguageAbstain(o.NearestLabel) {
					shadow.Abstain++
				}
				if hasRiskCode(ev.ReasonCodes) {
					shadow.RiskDecisions++
					if !o.WouldLift && ranksBelow(o.NearestLabel, o.RegexLabel) {
						shadow.FalseDowngradeProxy++
					}
				}
			}
		}
		if f := ev.E5Fuse; f != nil {
			if fuse == nil {
				fuse = &E5FuseSummary{Actions: map[string]int{}, ErrorCodes: map[string]int{}}
			}
			fuse.Observed++
			if f.ErrorCode != "" {
				fuse.Errors++
				fuse.ErrorCodes[f.ErrorCode]++
				continue
			}
			fuseLat = append(fuseLat, f.LatencyMs)
			if f.Action != "" {
				fuse.Actions[f.Action]++
			}
		}
	}
	if shadow != nil {
		shadow.WouldLiftRate = ratio(shadow.WouldLift, shadow.Scored)
		shadow.AbstainShare = ratio(shadow.Abstain, shadow.Scored)
		shadow.Latency = latencyStats(shadowLat)
		for _, r := range byRegex {
			r.WouldLiftRate = ratio(r.WouldLift, r.Scored)
			shadow.ByRegexLabel = append(shadow.ByRegexLabel, *r)
		}
		sort.Slice(shadow.ByRegexLabel, func(i, j int) bool {
			return labelLess(shadow.ByRegexLabel[i].RegexLabel, shadow.ByRegexLabel[j].RegexLabel)
		})
		if len(shadow.ErrorCodes) == 0 {
			shadow.ErrorCodes = nil
		}
		if len(shadow.NearestLabels) == 0 {
			shadow.NearestLabels = nil
		}
	}
	if fuse != nil {
		fuse.Latency = latencyStats(fuseLat)
		if len(fuse.Actions) == 0 {
			fuse.Actions = nil
		}
		if len(fuse.ErrorCodes) == 0 {
			fuse.ErrorCodes = nil
		}
	}
	return shadow, fuse
}

func hasRiskCode(codes []string) bool {
	for _, c := range codes {
		if c == telemetry.ReasonRiskCategory || c == telemetry.ReasonRiskFloor {
			return true
		}
	}
	return false
}

// complexityRank orders the classifier's labels; -1 means the label is not a
// complexity class (reserved ABSTAIN/FOREIGN, empty, or unknown).
func complexityRank(label string) int {
	switch label {
	case "TRIVIAL":
		return 0
	case "SIMPLE":
		return 1
	case "MEDIUM":
		return 2
	case "COMPLEX":
		return 3
	default:
		return -1
	}
}

// ranksBelow is true only when both labels are complexity classes and a
// ranks strictly below b.
func ranksBelow(a, b string) bool {
	ra, rb := complexityRank(a), complexityRank(b)
	return ra >= 0 && rb >= 0 && ra < rb
}

// labelLess sorts complexity classes in rank order, then the reserved labels,
// then anything else by name, so text and JSON read the same way every run.
func labelLess(a, b string) bool {
	ka, kb := labelKey(a), labelKey(b)
	if ka != kb {
		return ka < kb
	}
	return a < b
}

func labelKey(label string) int {
	if r := complexityRank(label); r >= 0 {
		return r
	}
	switch label {
	case e5shadow.LabelAbstain:
		return 4
	case e5shadow.LabelForeign:
		return 5
	}
	return 6
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func latencyStats(samples []float64) LatencyMs {
	if len(samples) == 0 {
		return LatencyMs{}
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	return LatencyMs{
		Samples: len(sorted),
		P50:     nearestRank(sorted, 0.50),
		P95:     nearestRank(sorted, 0.95),
		Max:     sorted[len(sorted)-1],
	}
}

// nearestRank is the nearest-rank percentile of an ascending sample.
func nearestRank(sorted []float64, p float64) float64 {
	i := int(math.Ceil(p*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

// sortedKeys returns histogram keys in label order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return labelLess(keys[i], keys[j]) })
	return keys
}

func histogramLine(m map[string]int) string {
	parts := make([]string, 0, len(m))
	for _, k := range sortedKeys(m) {
		name := k
		if name == "" {
			name = "(empty)"
		}
		parts = append(parts, fmt.Sprintf("%s %d", name, m[k]))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// actionHistogramLine prints fuse actions in the order the fuse documents
// them (none, abstain, raise), then any other value by name.
func actionHistogramLine(m map[string]int) string {
	order := map[string]int{"none": 0, "abstain": 1, "raise": 2}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, iok := order[keys[i]]
		oj, jok := order[keys[j]]
		switch {
		case iok && jok:
			return oi < oj
		case iok != jok:
			return iok
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func pct100(f float64) string { return fmt.Sprintf("%5.1f%%", f*100) }

// writeText prints the E5 shadow section; nothing when s is nil.
func (s *E5ShadowSummary) writeText(w io.Writer) {
	if s == nil {
		return
	}
	fmt.Fprintf(w, "E5 shadow\n")
	fmt.Fprintf(w, "  observed            %8d  (decision events with e5_shadow)\n", s.Observed)
	fmt.Fprintf(w, "  helper errors       %8d  %s\n", s.Errors, histogramLine(s.ErrorCodes))
	fmt.Fprintf(w, "  scored              %8d  (helper answered; denominator of the rates below)\n", s.Scored)
	fmt.Fprintf(w, "  would_lift          %8d  %s\n", s.WouldLift, pct100(s.WouldLiftRate))
	for _, r := range s.ByRegexLabel {
		name := r.RegexLabel
		if name == "" {
			name = "(empty)"
		}
		fmt.Fprintf(w, "    regex %-9s     %8d  %s  of %d\n", name, r.WouldLift, pct100(r.WouldLiftRate), r.Scored)
	}
	fmt.Fprintf(w, "  nearest label                 %s\n", histogramLine(s.NearestLabels))
	fmt.Fprintf(w, "  abstain             %8d  %s  (nearest ABSTAIN or FOREIGN)\n", s.Abstain, pct100(s.AbstainShare))
	fmt.Fprintf(w, "  risk decisions      %8d  (RISK_CATEGORY or RISK_FLOOR)\n", s.RiskDecisions)
	fmt.Fprintf(w, "  false downgrade proxy %6d  (risk decision, no lift, nearest below regex; a proxy, not a reviewed error)\n", s.FalseDowngradeProxy)
	fmt.Fprintf(w, "  latency ms          p50 %g  p95 %g  max %g  (%d scored)\n", s.Latency.P50, s.Latency.P95, s.Latency.Max, s.Latency.Samples)
}

// writeText prints the E5 fuse section; nothing when f is nil.
func (f *E5FuseSummary) writeText(w io.Writer) {
	if f == nil {
		return
	}
	fmt.Fprintf(w, "E5 fuse\n")
	fmt.Fprintf(w, "  observed            %8d  (decision events with e5_fuse)\n", f.Observed)
	fmt.Fprintf(w, "  actions                       %s\n", actionHistogramLine(f.Actions))
	fmt.Fprintf(w, "  helper errors       %8d  %s\n", f.Errors, histogramLine(f.ErrorCodes))
	fmt.Fprintf(w, "  latency ms          p50 %g  p95 %g  max %g  (%d answered)\n", f.Latency.P50, f.Latency.P95, f.Latency.Max, f.Latency.Samples)
}
