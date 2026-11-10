// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// DOMAIN: Mobile and native build tools.

// Detox family constants.
const (
	FamilyDetoxTest  Family = "detox-test"
	FamilyDetoxBuild Family = "detox-build"
)

// mobileNativeFamily detects mobile/native build tools, npx forms included.
// The compactor buffers output until the process exits, so every form that
// never exits (start, log-*, a streaming adb logcat, any --watch) is refused,
// as is every form that publishes or exports something. A remote URL
// argument is refused too.
func mobileNativeFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	prog, args := argv[0], argv[1:]
	if prog == "npx" {
		if len(args) == 0 {
			return FamilyNone
		}
		var ok bool
		if prog, ok = npxTool(args[0]); !ok {
			return FamilyNone
		}
		args = args[1:]
	}
	for _, a := range args {
		if strings.Contains(a, "://") || strings.HasPrefix(a, "--watch") || strings.HasPrefix(a, "--inspect") {
			return FamilyNone
		}
	}
	switch prog {
	case "xcodebuild":
		return xcodebuildFamily(args)
	case "pod":
		return podFamily(args)
	case "expo":
		return expoFamily(args)
	case "react-native":
		return reactNativeFamily(args)
	case "detox":
		return detoxFamily(args)
	case "adb":
		if argv[0] == "adb" {
			return adbFamily(args)
		}
	}
	return FamilyNone
}

// npxVersion is the only version suffix npx may carry: a plain version or
// dist-tag. A URL, git, file or path spec (':' or '/') would fetch other code.
var npxVersion = lazyre.New(`^[0-9A-Za-z.\-]+$`)

// npxTools are the only packages npx may run here. xcodebuild, pod and adb are
// native tools: `npx xcodebuild` would fetch an npm package of that name.
var npxTools = map[string]bool{"expo": true, "react-native": true, "detox": true}

// npxTool strips a version suffix (expo@52 becomes expo) and reports false for
// a tool outside npxTools or a version that is not a plain version string.
func npxTool(name string) (string, bool) {
	tool, version, hasVersion := strings.Cut(name, "@")
	if !npxTools[tool] {
		return "", false
	}
	if hasVersion && !npxVersion.MatchString(version) {
		return "", false
	}
	return tool, true
}

// xcodebuildDenied lists words that archive, export, install, clean, download
// or sign, wherever they appear.
var xcodebuildDenied = map[string]bool{
	"clean": true, "archive": true, "install": true, "installsrc": true,
	"installhdrs": true, "installapi": true, "docbuild": true,
	"-exportArchive": true, "-exportNotarizedApp": true, "-exportLocalizations": true,
	"-importLocalizations": true, "-create-xcframework": true,
	"-resolvePackageDependencies": true, "-downloadPlatform": true,
	"-downloadAllPlatforms": true, "-runFirstLaunch": true,
	"-allowProvisioningUpdates": true, "-allowProvisioningDeviceRegistration": true,
	"-downloadComponent": true, "-importComponent": true, "-exportPlatform": true,
	"-importPlatform": true, "-license": true,
}

// xcodebuildFlagsWithValue are the flags whose next word is a value, never an
// action (`-scheme test` names a scheme).
var xcodebuildFlagsWithValue = map[string]bool{
	"-scheme": true, "-project": true, "-workspace": true, "-target": true,
	"-configuration": true, "-sdk": true, "-destination": true, "-arch": true,
	"-derivedDataPath": true, "-resultBundlePath": true, "-testPlan": true,
	"-xctestrun": true, "-archivePath": true, "-toolchain": true,
	"-only-testing": true, "-skip-testing": true, "-jobs": true,
	"-resultStreamPath": true, "-clonedSourcePackagesDirPath": true,
	"-packageCachePath": true, "-testLanguage": true, "-testRegion": true,
	"-maximum-concurrent-test-device-destinations":    true,
	"-maximum-concurrent-test-simulator-destinations": true,
	"-default-test-execution-time-allowance":          true,
	"-maximum-test-execution-time-allowance":          true,
}

// xcodebuildFamily accepts build, test and inspection actions only. Anything
// that archives, exports, installs, cleans or downloads is refused wherever it
// appears; the action is taken only from words that are not a flag's value.
func xcodebuildFamily(args []string) Family {
	accepted := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if xcodebuildDenied[a] {
			return FamilyNone
		}
		if xcodebuildFlagsWithValue[a] {
			i++
			continue
		}
		switch a {
		case "build", "test", "build-for-testing", "test-without-building",
			"-list", "-showBuildSettings":
			accepted = true
		}
	}
	if accepted {
		return FamilyXcodebuild
	}
	return FamilyNone
}

// podFamily accepts dependency installs and read-only forms. Repo publishing,
// trunk and every spec form except lint are refused.
func podFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	switch args[0] {
	case "install", "update", "outdated", "list":
		return FamilyPodInstall
	case "spec":
		if len(args) >= 2 && args[1] == "lint" {
			return FamilyPodInstall
		}
	case "repo":
		if len(args) >= 2 && (args[1] == "update" || args[1] == "list") {
			return FamilyPodInstall
		}
	}
	return FamilyNone
}

// expoFamily accepts project preparation and checks. install stays, as
// npm install is accepted; upgrade rewrites the project, and start and
// run:* never exit, so they are refused.
func expoFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	switch args[0] {
	case "prebuild", "export", "doctor", "install", "config", "diagnostics":
		return FamilyExpo
	}
	return FamilyNone
}

func reactNativeFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	switch args[0] {
	case "run-android", "run-ios", "bundle", "info", "doctor":
		return FamilyReactNative
	}
	return FamilyNone
}

func detoxFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	switch args[0] {
	case "test":
		return FamilyDetoxTest
	case "build":
		return FamilyDetoxBuild
	}
	return FamilyNone
}

// adbFamily keeps install (a build step, like gradle installDebug) and
// listing. logcat is accepted only when it dumps and exits (-d, -t N).
// File transfer and bugreport can wait on a device and are refused.
func adbFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	switch args[0] {
	case "install", "devices":
		return FamilyAdb
	case "logcat":
		return adbLogcatFamily(args[1:])
	}
	return FamilyNone
}

// adbLogcatFamily accepts a logcat that dumps and exits: `-d` as its own
// word, `--dump`, or `-t N`. Flags that clear, resize, send commands or write
// a file (-c, -G, -P, -f, -e, --regex) are refused, so `-e -d` (where -d is the
// regex) cannot pass.
func adbLogcatFamily(args []string) Family {
	exits := false
	for _, a := range args {
		switch {
		case a == "-c" || a == "-G" || a == "-P" || a == "-f" || a == "-e" || strings.HasPrefix(a, "--regex"):
			return FamilyNone
		case a == "-d" || a == "-t" || a == "--dump":
			exits = true
		}
	}
	if exits {
		return FamilyAdb
	}
	return FamilyNone
}
