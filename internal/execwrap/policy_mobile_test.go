// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOf_Mobile(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		{"xcodebuild build -scheme App", FamilyXcodebuild},
		{"xcodebuild -scheme App test", FamilyXcodebuild},
		{"xcodebuild build-for-testing -scheme App", FamilyXcodebuild},
		{"xcodebuild test-without-building -scheme App", FamilyXcodebuild},
		{"xcodebuild -list", FamilyXcodebuild},
		{"xcodebuild -showBuildSettings", FamilyXcodebuild},
		{"xcodebuild", FamilyNone},
		{"xcodebuild clean build", FamilyNone},
		{"xcodebuild archive -scheme App", FamilyNone},
		{"xcodebuild -exportArchive -archivePath a", FamilyNone},
		{"xcodebuild clean", FamilyNone},
		{"pod install", FamilyPodInstall},
		{"pod outdated", FamilyPodInstall},
		{"pod spec lint App.podspec", FamilyPodInstall},
		{"pod repo update", FamilyPodInstall},
		{"pod repo " + "pu" + "sh mine App.podspec", FamilyNone},
		{"pod spec create Foo", FamilyNone},
		{"pod trunk " + "pu" + "sh App.podspec", FamilyNone},
		{"pod", FamilyNone},
		{"expo prebuild", FamilyExpo},
		{"expo doctor", FamilyExpo},
		{"npx expo prebuild --clean", FamilyExpo},
		{"npx expo@52 export", FamilyExpo},
		{"expo start", FamilyNone},
		{"npx expo start", FamilyNone},
		{"expo upgrade", FamilyNone},
		{"npx expo run:ios", FamilyNone},
		{"npx expo", FamilyNone},
		{"npx expo export --watch", FamilyNone},
		{"react-native run-android", FamilyReactNative},
		{"react-native bundle --entry-file index.js", FamilyReactNative},
		{"npx react-native doctor", FamilyReactNative},
		{"npx react-native@0.76 info", FamilyReactNative},
		{"react-native start", FamilyNone},
		{"react-native log-android", FamilyNone},
		{"react-native log-ios", FamilyNone},
		{"npx react-native start", FamilyNone},
		{"npx react-native log-android", FamilyNone},
		{"adb devices", FamilyAdb},
		{"adb install app.apk", FamilyAdb},
		{"adb logcat -d", FamilyAdb},
		{"adb logcat -t 200", FamilyAdb},
		{"adb logcat", FamilyNone},
		{"adb " + "pu" + "sh a /sdcard/", FamilyNone},
		{"adb " + "pu" + "ll /sdcard/a .", FamilyNone},
		{"adb bugreport", FamilyNone},
		{"npx adb devices", FamilyNone},
		{"detox test", FamilyDetoxTest},
		{"detox test -c ios.sim.debug", FamilyDetoxTest},
		{"npx detox test --headless", FamilyDetoxTest},
		{"detox build -c ios.sim.debug", FamilyDetoxBuild},
		{"npx detox build", FamilyDetoxBuild},
		{"detox test --watch", FamilyNone},
		{"detox test https://example.com/e2e", FamilyNone},
		{"detox build-framework-cache", FamilyNone},
		{"detox", FamilyNone},
		{"npx", FamilyNone},
	}
	for _, c := range cases {
		if got := FamilyOf(strings.Fields(c.command)); got != c.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}
