// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

// applyFormatCompression runs the compressor of format f over input.
func applyFormatCompression(input []byte, f Format) ([]byte, bool, string) {
	switch f {
	case FormatGoTest:
		return compressGoTest(input)
	case FormatPytest:
		return compressPytest(input)
	case FormatVitest:
		return compressVitest(input)
	case FormatPlaywright:
		return compressPlaywright(input)
	case FormatCargoTest:
		return compressCargoTest(input)
	case FormatTSC:
		return compressTSC(input)
	case FormatJest:
		return compressJest(input)
	case FormatCypress:
		return compressCypress(input)
	case FormatNextBuild:
		return compressNextBuild(input)
	case FormatPHPUnit:
		return compressPHPUnit(input)
	case FormatFlutterTest:
		return compressFlutterTest(input)
	case FormatMaven:
		return compressMaven(input)
	case FormatGradle:
		return compressGradle(input)
	case FormatRspec:
		return compressRspec(input)
	case FormatDotnetTest:
		return compressDotnetTest(input)
	case FormatNpmAudit:
		return compressNpmAudit(input)
	case FormatNpmInstall:
		return compressNpmInstall(input)
	case FormatPipInstall:
		return compressPipInstall(input)
	case FormatGitStatus:
		return compressGitStatus(input)
	case FormatGitLog:
		return compressGitLog(input)
	case FormatGitDiff:
		return compressGitDiff(input)
	case FormatLockfileDiff:
		return compressLockfileDiff(input)
	case FormatGitShow:
		return compressGitShow(input)
	case FormatGitStash:
		return compressGitStashList(input)
	case FormatGhPr:
		return compressGhPrView(input)
	case FormatMarkdown:
		return compressMarkdownView(input)
	case FormatGhChecks:
		return compressGhPrChecks(input)
	case FormatGhRunList:
		return compressGhRunList(input)
	case FormatGhRunLog:
		return compressGhRunLog(input)
	case FormatSearch:
		return compressSearchResults(input)
	case FormatFileList:
		return compressFileList(input)

	// Command-directed listings: only CompressCommandOutput reaches these.
	case FormatLs:
		return compressLs(input)
	case FormatFind:
		return compressFind(input)
	case FormatGrep:
		return compressGrep(input)
	case FormatLogs:
		return deduplicateLogs(input)
	case FormatDockerPs:
		return compressDockerPs(input)
	case FormatDockerImages:
		return compressDockerImages(input)
	case FormatDockerLogs:
		return compressDockerLogs(input)
	case FormatKubectlPods:
		return compressKubectlGetPods(input)
	case FormatKubectlLogs:
		return compressKubectlLogs(input)

	// Linters
	case FormatEslint:
		return compressEslint(input)
	case FormatRuff:
		return compressRuff(input)
	case FormatRubocop:
		return compressRubocop(input)
	case FormatMypy:
		return compressMypy(input)
	case FormatGolangciLint:
		return compressGolangciLint(input)
	case FormatCargoClippy:
		return compressCargoClippy(input)

	// Build errors
	case FormatCargoBuild:
		return compressCargoBuild(input)
	case FormatGoBuild:
		return compressGoBuild(input)
	case FormatDotnetBuild:
		return compressDotnetBuild(input)

	// Docker build/push
	case FormatDockerBuild:
		return compressDockerBuild(input)
	case FormatDockerPush:
		return compressDockerPush(input)

	// Terraform
	case FormatTerraformPlan:
		return compressTerraformPlan(input)

	// Dependency trees
	case FormatNpmLs:
		return compressNpmLs(input)
	case FormatCargoTree:
		return compressCargoTree(input)
	case FormatGoModGraph:
		return compressGoModGraph(input)

	// PHP ecosystem
	case FormatComposer:
		return compressComposer(input)
	case FormatPHPStan:
		return compressPHPStan(input)
	case FormatPsalm:
		return compressPsalm(input)
	case FormatArtisanMigrate:
		return compressArtisanMigrate(input)
	case FormatArtisanRoutes:
		return compressArtisanRouteList(input)

	// Build systems and monorepo runners
	case FormatNx:
		return compressNx(input)
	case FormatTurbo:
		return compressTurbo(input)
	case FormatBazel:
		return compressBazel(input)

	// Azure DevOps
	case FormatAzPipelines:
		return compressAzPipelines(input)

	default:
		return compressGeneric(input)
	}
}
