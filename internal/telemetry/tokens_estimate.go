// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

func share(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

// TokensSavedEstimate is the one final figure: the tokens the Compactor kept
// out of the context, estimated from bytes / execwrap.BytesPerToken by the
// caller, who owns the exec log. Only these are tokens that were not spent.
// The tokens routing moved to a lower tier ran all the same, so they are not
// part of it (owner decision 2026-10-31: a lower tier is not fewer tokens);
// RoutingTokens names them apart. Zero when the Compactor saved nothing.
func TokensSavedEstimate(compactorTokens int64) int64 {
	if compactorTokens < 0 {
		return 0
	}
	return compactorTokens
}

// TokensSavedEstimateLine is the final text: `Tokens saved (estimate): 12.3k`.
func TokensSavedEstimateLine(total int64) string {
	return "Tokens saved (estimate): " + formatTokens(total)
}

// ShortTokens is the CLI token format without a label: "0", "999", "15k", "20.7M".
func ShortTokens(n int64) string {
	return formatTokens(n)
}
