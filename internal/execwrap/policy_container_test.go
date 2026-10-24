// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecideContainerExec(t *testing.T) {
	const pre = "'/opt/ds/downshift' compact -- "
	cases := []struct {
		name    string
		command string
		rewrite bool
		family  Family
		want    string
		reason  string
	}{
		{"compose exec phpunit", "docker compose exec -T app vendor/bin/phpunit", true, FamilyPHPUnit, pre + "docker compose exec -T app vendor/bin/phpunit", ReasonAllowlisted},
		{"compose exec artisan test", "docker compose exec -T app php artisan test", true, FamilyArtisan, pre + "docker compose exec -T app php artisan test", ReasonAllowlisted},
		{"compose exec no -T", "docker compose exec app vendor/bin/phpunit", true, FamilyPHPUnit, pre + "docker compose exec app vendor/bin/phpunit", ReasonAllowlisted},
		{"docker-compose exec", "docker-compose exec app vendor/bin/phpunit", true, FamilyPHPUnit, pre + "docker-compose exec app vendor/bin/phpunit", ReasonAllowlisted},
		{"docker exec npm test", "docker exec my-container npm test", true, FamilyNpmTest, pre + "docker exec my-container npm test", ReasonAllowlisted},
		{"docker exec -i go test", "docker exec -i app go test ./...", true, FamilyGoTest, pre + "docker exec -i app go test ./...", ReasonAllowlisted},
		{"short user and workdir", "docker compose exec -T -u www-data -w /var/www app vendor/bin/phpunit tests/Unit", true, FamilyPHPUnit, pre + "docker compose exec -T -u www-data -w /var/www app vendor/bin/phpunit tests/Unit", ReasonAllowlisted},
		{"long flags", "docker compose exec --no-TTY --user root --workdir /app --env APP_ENV=testing app vendor/bin/phpunit", true, FamilyPHPUnit, pre + "docker compose exec --no-TTY --user root --workdir /app --env APP_ENV=testing app vendor/bin/phpunit", ReasonAllowlisted},
		{"env -e", "docker exec -e CI=1 app npm test", true, FamilyNpmTest, pre + "docker exec -e CI=1 app npm test", ReasonAllowlisted},
		{"compound cd prefix", "cd /repo && docker compose exec -T app vendor/bin/phpunit", true, FamilyPHPUnit, "cd /repo && " + pre + "docker compose exec -T app vendor/bin/phpunit", ReasonAllowlistedCompound},
		{"compound tail suffix", "docker compose exec -T app vendor/bin/phpunit | tail -5", true, FamilyPHPUnit, pre + "docker compose exec -T app vendor/bin/phpunit | tail -5", ReasonAllowlistedCompound},

		{"inner bash not allowlisted", "docker compose exec app bash", false, "", "", ReasonNotAllowlisted},
		{"inner artisan migrate", "docker compose exec -T app php artisan migrate", false, "", "", ReasonNotAllowlisted},
		{"nested docker", "docker exec app docker compose exec app npm test", false, "", "", ReasonNotAllowlisted},
		{"tty -it refused", "docker compose exec -it app vendor/bin/phpunit", false, "", "", ReasonNotAllowlisted},
		{"tty -t refused", "docker exec -t app npm test", false, "", "", ReasonNotAllowlisted},
		{"detach refused", "docker exec -d app npm test", false, "", "", ReasonNotAllowlisted},
		{"unknown compose flag", "docker compose exec --index 2 -T app vendor/bin/phpunit", false, "", "", ReasonNotAllowlisted},
		{"unknown long flag", "docker compose exec --privileged -T app npm test", false, "", "", ReasonNotAllowlisted},
		{"option value is a flag", "docker compose exec -u -T app npm test", false, "", "", ReasonNotAllowlisted},
		{"no service", "docker compose exec -T", false, "", "", ReasonNotAllowlisted},
		{"no inner command", "docker exec app", false, "", "", ReasonNotAllowlisted},
		{"env without equals", "docker exec -e CI app npm test", false, "", "", ReasonNotAllowlisted},
		{"quoted env value", "docker exec -e 'CI=1' app npm test", false, "", "", ReasonUnsafeChar},
		{"docker run unchanged", "docker run --rm node:20 npm test", false, "", "", ReasonNotAllowlisted},
		{"compose up unchanged", "docker compose up -d", false, "", "", ReasonNotAllowlisted},
		{"compose ps unchanged", "docker compose ps", false, "", "", ReasonNotAllowlisted},
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

// TestRun_ContainerExecKeepsExitCode runs a fake docker that prints a PHPUnit
// transcript. A zero exit compacts under the inner family; a non-zero exit
// keeps the raw output and the real exit code.
func TestRun_ContainerExecKeepsExitCode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", "phpunit_pass_default.txt"))
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Fields("docker compose exec -T app vendor/bin/phpunit")

	fakeBin(t, "docker", "cat <<'EOF'\n"+string(data)+"EOF\nexit 0")
	r, stdout, stderr := run(t, argv, true)
	if r.ExitCode != 0 || !r.Compacted || r.Family != FamilyPHPUnit || stderr != "" || !strings.Contains(stdout, "PHPUnit: PASS (58 tests, 143 assertions") {
		t.Fatalf("pass: result=%+v stdout=%q stderr=%q", r, stdout, stderr)
	}

	fakeBin(t, "docker", "cat <<'EOF'\n"+string(data)+"EOF\nexit 3")
	r, stdout, _ = run(t, argv, true)
	if r.ExitCode != 3 || r.Compacted || r.Family != FamilyPHPUnit || stdout != string(data) {
		t.Fatalf("fail: result=%+v stdout=%q", r, stdout)
	}
}
