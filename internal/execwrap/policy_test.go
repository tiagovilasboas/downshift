// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

const exe = "/opt/ds/downshift"

func TestDecide(t *testing.T) {
	cases := []struct {
		name    string
		command string
		rewrite bool
		family  Family
		want    string // wrapped command when rewrite
		reason  string
	}{
		{"go test", "go test ./...", true, FamilyGoTest, "'/opt/ds/downshift' compact -- go test ./...", ReasonAllowlisted},
		{"go test flags", "go test -count=1 -run TestX ./internal/core", true, FamilyGoTest, "'/opt/ds/downshift' compact -- go test -count=1 -run TestX ./internal/core", ReasonAllowlisted},
		{"git status", "git status", true, FamilyGitStatus, "'/opt/ds/downshift' compact -- git status", ReasonAllowlisted},
		{"surrounding space", "  git status  ", true, FamilyGitStatus, "'/opt/ds/downshift' compact -- git status", ReasonAllowlisted},
		{"collapsed spaces", "go  test   ./...", true, FamilyGoTest, "'/opt/ds/downshift' compact -- go test ./...", ReasonAllowlisted},
		{"pytest bare", "pytest", true, FamilyPytest, "'/opt/ds/downshift' compact -- pytest", ReasonAllowlisted},
		{"pytest with args", "pytest -v tests/test_app.py", true, FamilyPytest, "'/opt/ds/downshift' compact -- pytest -v tests/test_app.py", ReasonAllowlisted},
		{"vitest bare", "vitest", true, FamilyVitest, "'/opt/ds/downshift' compact -- vitest", ReasonAllowlisted},
		{"vitest run", "vitest run", true, FamilyVitest, "'/opt/ds/downshift' compact -- vitest run", ReasonAllowlisted},
		{"npx vitest", "npx vitest", true, FamilyVitest, "'/opt/ds/downshift' compact -- npx vitest", ReasonAllowlisted},
		{"npx vitest run", "npx vitest run --reporter=verbose", true, FamilyVitest, "'/opt/ds/downshift' compact -- npx vitest run --reporter=verbose", ReasonAllowlisted},
		{"jest bare", "jest", true, FamilyJest, "'/opt/ds/downshift' compact -- jest", ReasonAllowlisted},
		{"jest run", "jest --runInBand", true, FamilyJest, "'/opt/ds/downshift' compact -- jest --runInBand", ReasonAllowlisted},
		{"npx jest", "npx jest", true, FamilyJest, "'/opt/ds/downshift' compact -- npx jest", ReasonAllowlisted},
		{"cypress run", "cypress run", true, FamilyCypress, "'/opt/ds/downshift' compact -- cypress run", ReasonAllowlisted},
		{"npx cypress run", "npx cypress run", true, FamilyCypress, "'/opt/ds/downshift' compact -- npx cypress run", ReasonAllowlisted},
		{"next build", "next build", true, FamilyNextBuild, "'/opt/ds/downshift' compact -- next build", ReasonAllowlisted},
		{"npx next build", "npx next build", true, FamilyNextBuild, "'/opt/ds/downshift' compact -- npx next build", ReasonAllowlisted},
		{"phpunit", "phpunit", true, FamilyPHPUnit, "'/opt/ds/downshift' compact -- phpunit", ReasonAllowlisted},
		{"vendor phpunit", "./vendor/bin/phpunit", true, FamilyPHPUnit, "'/opt/ds/downshift' compact -- ./vendor/bin/phpunit", ReasonAllowlisted},
		{"flutter test", "flutter test", true, FamilyFlutterTest, "'/opt/ds/downshift' compact -- flutter test", ReasonAllowlisted},
		{"flutter test args", "flutter test test/a_test.dart -r expanded", true, FamilyFlutterTest, "'/opt/ds/downshift' compact -- flutter test test/a_test.dart -r expanded", ReasonAllowlisted},
		{"dart test", "dart test --no-color", true, FamilyDartTest, "'/opt/ds/downshift' compact -- dart test --no-color", ReasonAllowlisted},
		{"flutter analyze not allowlisted", "flutter analyze", false, "", "", ReasonNotAllowlisted},
		{"dart run not allowlisted", "dart run", false, "", "", ReasonNotAllowlisted},
		{"flutter pub get not allowlisted", "flutter pub get", false, "", "", ReasonNotAllowlisted},
		{"gradle build", "gradle build", true, FamilyGradle, "'/opt/ds/downshift' compact -- gradle build", ReasonAllowlisted},
		{"gradlew test", "./gradlew test --info", true, FamilyGradle, "'/opt/ds/downshift' compact -- ./gradlew test --info", ReasonAllowlisted},
		{"gradlew qualified", "gradlew :app:assembleDebug", true, FamilyGradle, "'/opt/ds/downshift' compact -- gradlew :app:assembleDebug", ReasonAllowlisted},
		{"gradlew flag first", "./gradlew --no-daemon lintDebug", true, FamilyGradle, "'/opt/ds/downshift' compact -- ./gradlew --no-daemon lintDebug", ReasonAllowlisted},
		{"gradle exclude", "gradle build -x test", true, FamilyGradle, "'/opt/ds/downshift' compact -- gradle build -x test", ReasonAllowlisted},
		{"gradle several tasks", "./gradlew clean :app:assembleDebug", true, FamilyGradle, "'/opt/ds/downshift' compact -- ./gradlew clean :app:assembleDebug", ReasonAllowlisted},
		{"gradle clean publish", "./gradlew clean publish", false, "", "", ReasonNotAllowlisted},
		{"gradle exclude then publish", "gradle -x test publish", false, "", "", ReasonNotAllowlisted},
		{"gradle build then publish", "gradle build publish", false, "", "", ReasonNotAllowlisted},
		{"gradle only exclude", "gradle -x test", false, "", "", ReasonNotAllowlisted},
		{"gradle testPublish", "gradle testPublish", false, "", "", ReasonNotAllowlisted},
		{"gradle testDeploy", "./gradlew testDeploy", false, "", "", ReasonNotAllowlisted},
		{"gradle qualified testPublish", "./gradlew :app:testPublish", false, "", "", ReasonNotAllowlisted},
		{"gradle testDebugUnitTest", "./gradlew :app:testDebugUnitTest", true, FamilyGradle, "'/opt/ds/downshift' compact -- ./gradlew :app:testDebugUnitTest", ReasonAllowlisted},
		{"gradle testDebugPublish", "gradle testDebugPublish", false, "", "", ReasonNotAllowlisted},
		{"gradle assembleReleaseDeploy", "./gradlew assembleReleaseDeploy", false, "", "", ReasonNotAllowlisted},
		{"gradle lintDebugUpload", "./gradlew :app:lintDebugUpload", false, "", "", ReasonNotAllowlisted},
		{"gradle no task", "gradle", false, "", "", ReasonNotAllowlisted},
		{"gradle version", "gradle --version", false, "", "", ReasonNotAllowlisted},
		{"gradle other task", "gradle publish", false, "", "", ReasonNotAllowlisted},
		{"gradle dependencies", "./gradlew dependencies", false, "", "", ReasonNotAllowlisted},
		{"gradle option with value", "gradle -p app build", false, "", "", ReasonNotAllowlisted},

		{"git log not allowlisted", "git log", false, "", "", ReasonNotAllowlisted},
		{"git diff", "git diff", false, "", "", ReasonNotAllowlisted},
		{"go build", "go build ./...", false, "", "", ReasonNotAllowlisted},
		{"bare go", "go", false, "", "", ReasonNotAllowlisted},
		{"path to git", "/usr/bin/git status", false, "", "", ReasonNotAllowlisted},
		{"git global option first", "git -C /tmp status", false, "", "", ReasonNotAllowlisted},
		{"sudo", "sudo go test ./...", false, "", "", ReasonNotAllowlisted},
		{"env assignment", "GOFLAGS=-v go test ./...", false, "", "", ReasonNotAllowlisted},
		{"empty", "", false, "", "", ReasonEmpty},
		{"blank", "   ", false, "", "", ReasonEmpty},

		{"already wrapped", "downshift exec -- go test ./...", false, "", "", ReasonAlreadyWrapped},
		{"wrapped twice", "downshift exec -- downshift exec -- git status", false, "", "", ReasonAlreadyWrapped},

		{"pipe", "go test ./... | tail", false, "", "", ReasonUnsafeChar},
		{"semicolon", "git status; rm -rf x", false, "", "", ReasonUnsafeChar},
		{"and list", "go test ./... && echo ok", false, "", "", ReasonUnsafeChar},
		{"background", "go test ./... &", false, "", "", ReasonUnsafeChar},
		{"redirect out", "go test ./... > out.txt", false, "", "", ReasonUnsafeChar},
		{"redirect in", "git status < x", false, "", "", ReasonUnsafeChar},
		{"dollar", "go test $PKG", false, "", "", ReasonUnsafeChar},
		{"backtick", "go test `pwd`", false, "", "", ReasonUnsafeChar},
		{"subshell", "go test $(pwd)", false, "", "", ReasonUnsafeChar},
		{"parens", "(git status)", false, "", "", ReasonUnsafeChar},
		{"newline", "git status\nrm -rf x", false, "", "", ReasonUnsafeChar},
		{"carriage return", "git status\r", false, "", "", ReasonUnsafeChar},
		{"trailing newline", "git status\n", false, "", "", ReasonUnsafeChar},
		{"tab", "go\ttest ./...", false, "", "", ReasonUnsafeChar},
		{"double quote", "go test -run \"TestX\" ./...", false, "", "", ReasonUnsafeChar},
		{"single quote", "go test -run 'TestX' ./...", false, "", "", ReasonUnsafeChar},
		{"glob star", "go test ./internal/*", false, "", "", ReasonUnsafeChar},
		{"glob question", "go test ./interna?", false, "", "", ReasonUnsafeChar},
		{"glob bracket", "go test ./[a-z]", false, "", "", ReasonUnsafeChar},
		{"brace", "go test ./{a,b}", false, "", "", ReasonUnsafeChar},
		{"tilde", "go test ~/x", false, "", "", ReasonUnsafeChar},
		{"backslash", "go test .\\x", false, "", "", ReasonUnsafeChar},
		{"comment", "git status #x", false, "", "", ReasonUnsafeChar},
		{"bang", "git status !1", false, "", "", ReasonUnsafeChar},
		{"caret", "go test -run ^TestX", false, "", "", ReasonUnsafeChar},
		{"non ascii", "go test ./ação", false, "", "", ReasonUnsafeChar},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite != tc.rewrite || d.Family != tc.family || d.Reason != tc.reason {
				t.Fatalf("Decide(%q) = %+v, want rewrite=%v family=%q reason=%q", tc.command, d, tc.rewrite, tc.family, tc.reason)
			}
			if d.Command != tc.want {
				t.Fatalf("Command = %q, want %q", d.Command, tc.want)
			}
		})
	}
}

func TestDecide_TooLong(t *testing.T) {
	long := "go test " + string(make([]byte, 0))
	for len(long) <= maxCommandBytes {
		long += " ./x"
	}
	if d := Decide(long, exe); d.Rewrite || d.Reason != ReasonTooLong {
		t.Fatalf("Decide(long, exe) = %+v, want too_long", d)
	}
}

// A wrapped command must never be wrapped again: Decide on its own output
// always refuses.
func TestDecide_Idempotent(t *testing.T) {
	for _, c := range []string{"go test ./...", "git status"} {
		first := Decide(c, exe)
		if !first.Rewrite {
			t.Fatalf("Decide(%q) did not rewrite", c)
		}
		if second := Decide(first.Command, exe); second.Rewrite {
			t.Fatalf("Decide(%q) rewrote an already wrapped command: %+v", first.Command, second)
		}
	}
}

func TestDecide_BadExecutable(t *testing.T) {
	for _, bad := range []string{"", "downshift", "./downshift", "/opt/it's/downshift", "/opt/a\nb/downshift"} {
		if d := Decide("go test ./...", bad); d.Rewrite || d.Reason != ReasonBadExecutable {
			t.Fatalf("exe %q: %+v", bad, d)
		}
	}
	if q, ok := QuoteExecutable("/opt/my dir/$x/downshift"); !ok || q != "'/opt/my dir/$x/downshift'" {
		t.Fatalf("quote = %q %v", q, ok)
	}
}
