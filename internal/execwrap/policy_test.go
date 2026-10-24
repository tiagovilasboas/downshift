// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"testing"
)

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
		{"flutter analyze", "flutter analyze", true, FamilyFlutterAnalyze, "'/opt/ds/downshift' compact -- flutter analyze", ReasonAllowlisted},
		{"dart run", "dart run", true, FamilyDartRun, "'/opt/ds/downshift' compact -- dart run", ReasonAllowlisted},
		{"flutter pub get", "flutter pub get", true, FamilyFlutterPub, "'/opt/ds/downshift' compact -- flutter pub get", ReasonAllowlisted},
		{"flutter doctor", "flutter doctor", true, FamilyFlutterDoctor, "'/opt/ds/downshift' compact -- flutter doctor", ReasonAllowlisted},
		{"flutter build", "flutter build apk", true, FamilyFlutterBuild, "'/opt/ds/downshift' compact -- flutter build apk", ReasonAllowlisted},
		{"dart analyze", "dart analyze", true, FamilyDartAnalyze, "'/opt/ds/downshift' compact -- dart analyze", ReasonAllowlisted},
		{"dart pub", "dart pub get", true, FamilyDartPub, "'/opt/ds/downshift' compact -- dart pub get", ReasonAllowlisted},
		{"mvn test", "mvn test", true, FamilyMaven, "'/opt/ds/downshift' compact -- mvn test", ReasonAllowlisted},
		{"mvn clean verify", "mvn clean verify", true, FamilyMaven, "'/opt/ds/downshift' compact -- mvn clean verify", ReasonAllowlisted},
		{"mvn package flags", "mvn package -DskipTests -o", true, FamilyMaven, "'/opt/ds/downshift' compact -- mvn package -DskipTests -o", ReasonAllowlisted},
		{"mvnw install", "mvnw install", true, FamilyMaven, "'/opt/ds/downshift' compact -- mvnw install", ReasonAllowlisted},
		{"./mvnw test", "./mvnw test", true, FamilyMaven, "'/opt/ds/downshift' compact -- ./mvnw test", ReasonAllowlisted},
		{"mvn option first", "mvn -q test", false, "", "", ReasonNotAllowlisted},
		{"mvn plugin goal", "mvn spring-boot:run", false, "", "", ReasonNotAllowlisted},
		{"mvn deploy", "mvn deploy", false, "", "", ReasonNotAllowlisted},
		{"mvn clean deploy", "mvn clean deploy", false, "", "", ReasonNotAllowlisted},
		{"mvn settings file", "mvn clean install -s /tmp/s.xml", false, "", "", ReasonNotAllowlisted},
		{"mvn clean install", "mvn clean install -DskipTests", true, FamilyMaven, "'/opt/ds/downshift' compact -- mvn clean install -DskipTests", ReasonAllowlisted},
		{"mvn -f install", "mvn test -f install", false, "", "", ReasonNotAllowlisted},
		{"mvn -s value", "mvn test -s install", false, "", "", ReasonNotAllowlisted},
		{"mvn --file", "mvn test --file=pom.xml", false, "", "", ReasonNotAllowlisted},
		{"mvn -P profile", "mvn test -Pci", false, "", "", ReasonNotAllowlisted},
		{"mvn -D path", "mvn test -Dfoo=/etc/passwd", false, "", "", ReasonNotAllowlisted},
		{"bare mvn", "mvn", false, "", "", ReasonNotAllowlisted},
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

		{"git log", "git log --oneline -5", true, FamilyGitLog, "'/opt/ds/downshift' compact -- git log --oneline -5", ReasonAllowlisted},
		{"git diff", "git diff --stat", true, FamilyGitDiff, "'/opt/ds/downshift' compact -- git diff --stat", ReasonAllowlisted},
		{"git show", "git show HEAD", true, FamilyGitShow, "'/opt/ds/downshift' compact -- git show HEAD", ReasonAllowlisted},
		{"git branch", "git branch -vv", true, FamilyGitBranch, "'/opt/ds/downshift' compact -- git branch -vv", ReasonAllowlisted},
		{"git fetch", "git fetch origin", true, FamilyGitFetch, "'/opt/ds/downshift' compact -- git fetch origin", ReasonAllowlisted},
		{"git pull", "git pull --ff-only", true, FamilyGitPull, "'/opt/ds/downshift' compact -- git pull --ff-only", ReasonAllowlisted},
		{"git push", "git push origin main", true, FamilyGitPush, "'/opt/ds/downshift' compact -- git push origin main", ReasonAllowlisted},
		{"git commit", "git commit -m fix", true, FamilyGitCommit, "'/opt/ds/downshift' compact -- git commit -m fix", ReasonAllowlisted},
		{"git add", "git add -v src/main.go", true, FamilyGitAdd, "'/opt/ds/downshift' compact -- git add -v src/main.go", ReasonAllowlisted},
		{"git stash", "git stash list", true, FamilyGitStash, "'/opt/ds/downshift' compact -- git stash list", ReasonAllowlisted},
		{"git remote", "git remote -v", true, FamilyGitRemote, "'/opt/ds/downshift' compact -- git remote -v", ReasonAllowlisted},
		{"git tag", "git tag -l", true, FamilyGitTag, "'/opt/ds/downshift' compact -- git tag -l", ReasonAllowlisted},
		{"git config read", "git config --get user.name", true, FamilyGitConfig, "'/opt/ds/downshift' compact -- git config --get user.name", ReasonAllowlisted},
		{"git config write", "git config user.name fix", true, FamilyGitOther, "'/opt/ds/downshift' compact -- git config user.name fix", ReasonAllowlisted},
		{"git other subcommand", "git rev-parse HEAD", true, FamilyGitOther, "'/opt/ds/downshift' compact -- git rev-parse HEAD", ReasonAllowlisted},
		{"git bare", "git", false, "", "", ReasonNotAllowlisted},
		{"git version option", "git --version", false, "", "", ReasonNotAllowlisted},

		// Package manager wrappers
		{"npm test", "npm test", true, FamilyNpmTest, "'/opt/ds/downshift' compact -- npm test", ReasonAllowlisted},
		{"npm run test", "npm run test", true, FamilyNpmTest, "'/opt/ds/downshift' compact -- npm run test", ReasonAllowlisted},
		{"npm t", "npm t", true, FamilyNpmTest, "'/opt/ds/downshift' compact -- npm t", ReasonAllowlisted},
		{"npm run build", "npm run build", true, FamilyNpmBuild, "'/opt/ds/downshift' compact -- npm run build", ReasonAllowlisted},
		{"pnpm test", "pnpm test", true, FamilyPnpmTest, "'/opt/ds/downshift' compact -- pnpm test", ReasonAllowlisted},
		{"pnpm build", "pnpm build", true, FamilyPnpmBuild, "'/opt/ds/downshift' compact -- pnpm build", ReasonAllowlisted},
		{"yarn test", "yarn test", true, FamilyYarnTest, "'/opt/ds/downshift' compact -- yarn test", ReasonAllowlisted},
		{"yarn build", "yarn build", true, FamilyYarnBuild, "'/opt/ds/downshift' compact -- yarn build", ReasonAllowlisted},
		{"bun test", "bun test", true, FamilyBunTest, "'/opt/ds/downshift' compact -- bun test", ReasonAllowlisted},
		{"bun build", "bun build", true, FamilyBunBuild, "'/opt/ds/downshift' compact -- bun build", ReasonAllowlisted},

		// PHP ecosystem
		{"phpstan", "phpstan analyse", true, FamilyPHPStan, "'/opt/ds/downshift' compact -- phpstan analyse", ReasonAllowlisted},
		{"vendor phpstan", "./vendor/bin/phpstan analyse src/", true, FamilyPHPStan, "'/opt/ds/downshift' compact -- ./vendor/bin/phpstan analyse src/", ReasonAllowlisted},
		{"psalm", "psalm", true, FamilyPsalm, "'/opt/ds/downshift' compact -- psalm", ReasonAllowlisted},
		{"vendor psalm", "./vendor/bin/psalm --no-cache", true, FamilyPsalm, "'/opt/ds/downshift' compact -- ./vendor/bin/psalm --no-cache", ReasonAllowlisted},
		{"composer validate", "composer validate", true, FamilyComposer, "'/opt/ds/downshift' compact -- composer validate", ReasonAllowlisted},
		{"composer show", "composer show", true, FamilyComposer, "'/opt/ds/downshift' compact -- composer show", ReasonAllowlisted},
		{"composer audit", "composer audit", true, FamilyComposer, "'/opt/ds/downshift' compact -- composer audit", ReasonAllowlisted},
		{"composer install not allowed", "composer install", false, "", "", ReasonNotAllowlisted},
		{"artisan test", "php artisan test", true, FamilyArtisan, "'/opt/ds/downshift' compact -- php artisan test", ReasonAllowlisted},
		{"artisan route list", "php artisan route:list", true, FamilyArtisan, "'/opt/ds/downshift' compact -- php artisan route:list", ReasonAllowlisted},
		{"artisan migrate not allowed", "php artisan migrate", false, "", "", ReasonNotAllowlisted},

		// Linters
		{"eslint", "eslint src/", true, FamilyESLint, "'/opt/ds/downshift' compact -- eslint src/", ReasonAllowlisted},
		{"npx eslint", "npx eslint .", true, FamilyESLint, "'/opt/ds/downshift' compact -- npx eslint .", ReasonAllowlisted},
		{"prettier check", "prettier --check .", true, FamilyPrettier, "'/opt/ds/downshift' compact -- prettier --check .", ReasonAllowlisted},
		{"prettier write not allowed", "prettier --write .", false, "", "", ReasonNotAllowlisted},
		{"biome check", "biome check", true, FamilyBiome, "'/opt/ds/downshift' compact -- biome check", ReasonAllowlisted},
		{"biome lint", "biome lint src/", true, FamilyBiome, "'/opt/ds/downshift' compact -- biome lint src/", ReasonAllowlisted},

		// Kubernetes
		{"kubectl get pods", "kubectl get pods", true, FamilyKubectlGet, "'/opt/ds/downshift' compact -- kubectl get pods", ReasonAllowlisted},
		{"kubectl get all", "kubectl get all -n voomp-creators-back", true, FamilyKubectlGet, "'/opt/ds/downshift' compact -- kubectl get all -n voomp-creators-back", ReasonAllowlisted},
		{"kubectl describe pod", "kubectl describe pod my-pod", true, FamilyKubectlDescribe, "'/opt/ds/downshift' compact -- kubectl describe pod my-pod", ReasonAllowlisted},
		{"kubectl logs", "kubectl logs my-pod -f", true, FamilyKubectlLogs, "'/opt/ds/downshift' compact -- kubectl logs my-pod -f", ReasonAllowlisted},
		{"kubectl delete not allowed", "kubectl delete pod my-pod", false, "", "", ReasonNotAllowlisted},
		{"kubectl apply not allowed", "kubectl apply -f deploy.yaml", false, "", "", ReasonNotAllowlisted},

		// Helm
		{"helm list", "helm list -A", true, FamilyHelmList, "'/opt/ds/downshift' compact -- helm list -A", ReasonAllowlisted},
		{"helm status", "helm status my-release", true, FamilyHelmStatus, "'/opt/ds/downshift' compact -- helm status my-release", ReasonAllowlisted},
		{"helm install not allowed", "helm install my-release ./chart", false, "", "", ReasonNotAllowlisted},

		// Terraform
		{"terraform plan", "terraform plan", true, FamilyTerraformPlan, "'/opt/ds/downshift' compact -- terraform plan", ReasonAllowlisted},
		{"terraform fmt check", "terraform fmt -check", true, FamilyTerraformFmt, "'/opt/ds/downshift' compact -- terraform fmt -check", ReasonAllowlisted},
		{"terraform fmt no check not allowed", "terraform fmt", false, "", "", ReasonNotAllowlisted},
		{"terraform apply not allowed", "terraform apply", false, "", "", ReasonNotAllowlisted},

		// Azure CLI
		{"az aks list", "az aks list", true, FamilyAzList, "'/opt/ds/downshift' compact -- az aks list", ReasonAllowlisted},
		{"az storage account show", "az storage account show -n myaccount", true, FamilyAzShow, "'/opt/ds/downshift' compact -- az storage account show -n myaccount", ReasonAllowlisted},
		{"az aks delete not allowed", "az aks delete -n mycluster", false, "", "", ReasonNotAllowlisted},

		// AWS CLI
		{"aws ec2 describe-instances", "aws ec2 describe-instances", true, FamilyAwsDescribe, "'/opt/ds/downshift' compact -- aws ec2 describe-instances", ReasonAllowlisted},
		{"aws s3 ls", "aws s3 ls", true, FamilyAwsS3Ls, "'/opt/ds/downshift' compact -- aws s3 ls", ReasonAllowlisted},
		{"aws s3 ls bucket", "aws s3 ls s3://my-bucket/", true, FamilyAwsS3Ls, "'/opt/ds/downshift' compact -- aws s3 ls s3://my-bucket/", ReasonAllowlisted},
		{"aws s3 cp", "aws s3 cp s3://bucket/file.txt .", true, FamilyAwsS3Cp, "'/opt/ds/downshift' compact -- aws s3 cp s3://bucket/file.txt .", ReasonAllowlisted},
		{"aws logs tail", "aws logs tail /aws/lambda/my-fn --follow", true, FamilyAwsLogs, "'/opt/ds/downshift' compact -- aws logs tail /aws/lambda/my-fn --follow", ReasonAllowlisted},
		{"aws ecs list-clusters", "aws ecs list-clusters", true, FamilyAwsList, "'/opt/ds/downshift' compact -- aws ecs list-clusters", ReasonAllowlisted},
		{"aws lambda get-function", "aws lambda get-function --function-name my-fn", true, FamilyAwsGet, "'/opt/ds/downshift' compact -- aws lambda get-function --function-name my-fn", ReasonAllowlisted},
		{"aws ec2 terminate-instances not allowed", "aws ec2 terminate-instances --instance-ids i-123", false, "", "", ReasonNotAllowlisted},
		{"aws s3 rm not allowed", "aws s3 rm s3://bucket/file.txt", false, "", "", ReasonNotAllowlisted},

		// Docker (safe commands only)
		{"docker ps", "docker ps", true, FamilyDockerPS, "'/opt/ds/downshift' compact -- docker ps", ReasonAllowlisted},
		{"docker ps all", "docker ps -a", true, FamilyDockerPS, "'/opt/ds/downshift' compact -- docker ps -a", ReasonAllowlisted},
		{"docker logs", "docker logs my-container -f", true, FamilyDockerLogs, "'/opt/ds/downshift' compact -- docker logs my-container -f", ReasonAllowlisted},
		{"docker images", "docker images", true, FamilyDockerImages, "'/opt/ds/downshift' compact -- docker images", ReasonAllowlisted},
		{"docker build", "docker build -t myapp .", true, FamilyDockerBuild, "'/opt/ds/downshift' compact -- docker build -t myapp .", ReasonAllowlisted},
		{"docker rm not allowed", "docker rm my-container", false, "", "", ReasonNotAllowlisted},
		{"docker run not allowed", "docker run nginx", false, "", "", ReasonNotAllowlisted},

		{"go build", "go build ./...", true, FamilyGoBuild, "'/opt/ds/downshift' compact -- go build ./...", ReasonAllowlisted},
		{"bare go", "go", false, "", "", ReasonNotAllowlisted},
		{"path to git", "/usr/bin/git status", false, "", "", ReasonNotAllowlisted},
		{"git global option first", "git -C /tmp status", false, "", "", ReasonNotAllowlisted},
		{"git config injection", "git -c core.pager=evil log", false, "", "", ReasonNotAllowlisted},
		{"git exec path", "git --exec-path=/tmp/x status", false, "", "", ReasonNotAllowlisted},
		{"git dir override", "git --git-dir=/tmp/x log", false, "", "", ReasonNotAllowlisted},
		{"git env assignment", "GIT_DIR=/tmp/x git log", false, "", "", ReasonNotAllowlisted},
		{"git after sudo", "sudo git log", false, "", "", ReasonNotAllowlisted},
		{"sudo", "sudo go test ./...", false, "", "", ReasonNotAllowlisted},
		{"env assignment", "GOFLAGS=-v go test ./...", false, "", "", ReasonNotAllowlisted},
		{"empty", "", false, "", "", ReasonEmpty},
		{"blank", "   ", false, "", "", ReasonEmpty},

		{"already wrapped", "downshift exec -- go test ./...", false, "", "", ReasonAlreadyWrapped},
		{"wrapped twice", "downshift exec -- downshift exec -- git status", false, "", "", ReasonAlreadyWrapped},

		{"pipe", "go test ./... | tail", false, "", "", ReasonUnsafeChar},
		{"or list", "git status || rm -rf x", false, "", "", ReasonUnsafeChar},
		{"background list", "go test ./... & echo ok", false, "", "", ReasonUnsafeChar},
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

// A lone argv word that holds spaces and only safe bytes is split; every other
// argv comes back unchanged, so its refusal stands.
func TestSplitSingleWord(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want []string
	}{
		{"quoted command", []string{"go test ./..."}, []string{"go", "test", "./..."}},
		{"extra spaces", []string{"  git   status  "}, []string{"git", "status"}},
		{"already split", []string{"go", "test", "./..."}, []string{"go", "test", "./..."}},
		{"single word", []string{"go"}, []string{"go"}},
		{"empty", []string{}, []string{}},
		{"spaces only", []string{"   "}, []string{"   "}},
		{"tab inside", []string{"go\ttest ./..."}, []string{"go\ttest ./..."}},
		{"newline inside", []string{"git status\nrm -rf x"}, []string{"git status\nrm -rf x"}},
		{"double quote", []string{`go test -run "X" ./...`}, []string{`go test -run "X" ./...`}},
		{"dollar", []string{"go test $PKG"}, []string{"go test $PKG"}},
		{"pipe", []string{"go test ./... | tail"}, []string{"go test ./... | tail"}},
		{"semicolon", []string{"git status; id"}, []string{"git status; id"}},
		{"glob", []string{"go test ./internal/*"}, []string{"go test ./internal/*"}},
		{"redirect", []string{"go test ./... > out"}, []string{"go test ./... > out"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitSingleWord(tc.argv)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("SplitSingleWord(%q) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}
}
