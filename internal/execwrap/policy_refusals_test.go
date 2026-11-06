// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

// The families wired to a compressor accept the read, build and test forms and
// refuse every form that writes, publishes, watches or opens a server.
func TestFamilyOf_WiredFamiliesAcceptAndRefuse(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		{"rspec", FamilyRspec},
		{"rspec spec/models/invoice_spec.rb", FamilyRspec},
		{"rspec --init", FamilyNone},
		{"rspec -o report.txt", FamilyNone},
		{"rspec --out=report.txt", FamilyNone},

		{"rubocop", FamilyRubocop},
		{"rubocop app/ --format clang", FamilyRubocop},
		{"rubocop -a", FamilyNone},
		{"rubocop -A app/", FamilyNone},
		{"rubocop -aF", FamilyNone},
		{"rubocop --auto-correct", FamilyNone},
		{"rubocop --autocorrect-all", FamilyNone},
		{"rubocop -o out.txt", FamilyNone},
		{"rubocop --out=out.txt", FamilyNone},
		{"rubocop --auto-gen-config", FamilyNone},
		{"rubocop --format json", FamilyNone},
		{"rubocop -f json", FamilyNone},
		{"rubocop --format=simple", FamilyNone},

		{"dotnet test", FamilyDotnetTest},
		{"dotnet test --no-build -c Release", FamilyDotnetTest},
		{"dotnet test --diag log.txt", FamilyNone},
		{"dotnet test -d log.txt", FamilyNone},
		{"dotnet test --results-directory out", FamilyNone},

		{"phpstan", FamilyPHPStan},
		{"phpstan analyse src --level=8", FamilyPHPStan},
		{"vendor/bin/phpstan analyze", FamilyPHPStan},
		{"phpstan analyse --generate-baseline", FamilyNone},
		{"phpstan analyse --watch", FamilyNone},
		{"phpstan clear-result-cache", FamilyNone},

		{"docker build -t api .", FamilyDockerBuild},
		{"docker build --no-cache -f Dockerfile.dev .", FamilyDockerBuild},
		{"docker build --push -t api .", FamilyNone},
		{"docker build -o out .", FamilyNone},
		{"docker build --output=type=local,dest=out .", FamilyNone},
		{"docker build --secret id=npm,src=.npmrc .", FamilyNone},
		{"docker build https://example.com/org/repo.tar.gz", FamilyNone},
		{"docker build github.com/user/repo", FamilyNone},
		{"docker build git@gitlab.com:x/y", FamilyNone},
		{"docker build -t api git@gitlab.com:x/y.git#main", FamilyNone},
		{"docker build ./repo.git", FamilyNone},
		{"docker build --iidfile id.txt .", FamilyNone},
		{"docker build --iidfile=id.txt .", FamilyNone},
		{"docker build --metadata-file meta.json .", FamilyNone},
		{"docker build --metadata-file=meta.json .", FamilyNone},
		{"docker rm api", FamilyNone},
		{"docker rmi api", FamilyNone},
		{"docker run api", FamilyNone},

		{"docker ps", FamilyDockerPS},
		{"docker ps -a", FamilyDockerPS},
		{"docker images", FamilyDockerImages},
		{"docker images -a", FamilyDockerImages},

		{"kubectl get pods", FamilyKubectlGet},
		{"kubectl get pods -n prod", FamilyKubectlGet},
		{"kubectl get pods -w", FamilyNone},
		{"kubectl get pods --watch", FamilyNone},
		{"kubectl get pods --watch-only", FamilyNone},
		{"kubectl delete pod api", FamilyNone},
		{"kubectl apply -f x.yaml", FamilyNone},

		{"git stash list", FamilyGitStash},

		{"cargo clippy", FamilyCargoClippy},
		{"cargo clippy --all-targets -- -D warnings", FamilyCargoClippy},
		{"cargo clippy --fix", FamilyNone},
		{"cargo clippy --fix --allow-dirty", FamilyNone},
		{"terraform apply", FamilyNone},
	}
	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			if got := FamilyOf(strings.Fields(c.command)); got != c.want {
				t.Fatalf("FamilyOf(%q) = %q, want %q", c.command, got, c.want)
			}
		})
	}
}

// A container exec inherits the refusals of its inner command.
func TestFamilyOf_ContainerExecInheritsRefusals(t *testing.T) {
	if got := FamilyOf(strings.Fields("docker compose exec -T app rspec")); got != FamilyRspec {
		t.Fatalf("exec rspec = %q", got)
	}
	if got := FamilyOf(strings.Fields("docker compose exec -T app rspec --init")); got != FamilyNone {
		t.Fatalf("exec rspec --init = %q, want none", got)
	}
}

// Every wired family has a compressor format, so a refusal never hides a
// family that compacts.
func TestWiredFamiliesHaveFormats(t *testing.T) {
	for _, f := range []Family{FamilyRspec, FamilyRubocop, FamilyDotnetTest, FamilyPHPStan, FamilyDockerBuild,
		FamilyDockerPS, FamilyDockerImages, FamilyKubectlGet, FamilyGitStash} {
		if !compactibleFamily(f) {
			t.Errorf("%q has no expectedFormats entry", f)
		}
	}
}
