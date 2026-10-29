// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "fmt"

// savingsIntervalLine states the 95% interval of control minus routed tokens
// per completed task. When it includes zero the saving is not established.
func savingsIntervalLine(s *Savings) string {
	line := fmt.Sprintf("95%% interval of tokens saved per task (control minus routed): %s to %s",
		formatSignedTokens(s.PerTaskTokensLow), formatSignedTokens(s.PerTaskTokensHigh))
	if s.PerTaskTokensLow <= 0 && s.PerTaskTokensHigh >= 0 {
		return line + "; not established: the interval includes zero"
	}
	return line
}

func formatSignedTokens(v float64) string {
	if v < 0 {
		return "-" + formatTokens(int64(-v+0.5))
	}
	return "+" + formatTokens(int64(v+0.5))
}
