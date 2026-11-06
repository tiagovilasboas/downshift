// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func argvOf(cmd string) []string { return strings.Fields(cmd) }

// A psql write, a file read or write, a variable, a connection string or a
// backslash command is never wrapped as a read.
func TestPsqlOnlyPlainReadsAreFamilies(t *testing.T) {
	refused := []string{
		`psql -c delete_from_users`,
		`psql -c drop`,
		`psql -c select_1;drop_table_t`,
		`psql -f dump.sql`,
		`psql --file=dump.sql`,
		`psql -o out.txt -c select_1`,
		`psql -L log.txt -c select_1`,
		`psql -v ON_ERROR_STOP=1 -c select_1`,
		`psql -c select_1 -c select_2`,
		`psql -c \copy_t_to_x`,
		`psql postgres://u:p@host/db -c select_1`,
		`psql host=h options=-cx -c select_1`,
		`psql -c`,
		`psql`,
		`psql -h`,
		`psql --unknown -c select_1`,
	}
	for _, cmd := range refused {
		if got := FamilyOf(argvOf(cmd)); got != FamilyNone {
			t.Errorf("FamilyOf(%q) = %q, want none", cmd, got)
		}
	}
	for _, cmd := range []string{
		`psql -c select_1`,
		`psql -h db.example.com -p 5432 -U app -d shop -c select_1`,
		`psql -Atc select_1 shop`,
		`psql --host=db --dbname=shop --command=show_all`,
		`psql -l`,
		`psql --list`,
		`psql --version`,
		`psql -V`,
	} {
		if got := FamilyOf(argvOf(cmd)); got != FamilyPsql {
			t.Errorf("FamilyOf(%q) = %q, want psql", cmd, got)
		}
	}
}

// Statements with spaces, so they are argv slices.
func TestPsqlStatementsWithSpaces(t *testing.T) {
	refused := []string{
		"with x as (delete from t returning *) select 1", "explain analyze select 1", "select 1 into t",
		"select 1; drop table t", "delete from t", "update t set a = 1", "insert into t values (1)",
		"copy t to program 'x'", "truncate t", "create table t (a int)", "do $$ begin end $$",
	}
	for _, stmt := range refused {
		if got := FamilyOf([]string{"psql", "-c", stmt}); got != FamilyNone {
			t.Errorf("psql -c %q = %q, want none", stmt, got)
		}
	}
	for _, stmt := range []string{"select 1", "select * from users where id = 3", "show all", "explain select 1"} {
		if got := FamilyOf([]string{"psql", "-d", "shop", "-c", stmt}); got != FamilyPsql {
			t.Errorf("psql -c %q = %q, want psql", stmt, got)
		}
	}
}

func TestGcloudOnlyReadVerbsAreFamilies(t *testing.T) {
	refused := []string{
		`gcloud auth login`,
		`gcloud auth revoke --all`,
		`gcloud config set project p`,
		`gcloud compute ssh vm --command=rm_-rf_x`,
		`gcloud compute ssh vm`,
		`gcloud compute instances delete vm`,
		`gcloud compute instances describe vm --command=x`,
		`gcloud run deploy svc`,
		`gcloud auth print-access-token`,
		`gcloud compute instances list --impersonate-service-account=a@b`,
		`gcloud`,
		`gcloud version`,
	}
	for _, cmd := range refused {
		if got := FamilyOf(argvOf(cmd)); got == FamilyGcloud {
			t.Errorf("FamilyOf(%q) = gcloud, want none", cmd)
		}
	}
	for _, cmd := range []string{
		`gcloud compute instances describe vm --zone z`,
		`gcloud compute instances list`,
		`gcloud logging read severity>=ERROR`,
		`gcloud projects get-iam-policy p`,
		`gcloud container clusters list --format=json`,
		`gcloud config list`,
		`gcloud auth list`,
	} {
		want := FamilyGcloud
		if strings.Contains(cmd, "get-iam-policy") {
			want = FamilyNone // not a listed read verb: refused rather than guessed
		}
		if got := FamilyOf(argvOf(cmd)); got != want {
			t.Errorf("FamilyOf(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestAstGrepNeverUpdatesFiles(t *testing.T) {
	for _, cmd := range []string{
		`ast-grep run -p foo -r bar -U`,
		`ast-grep run -p foo -r bar --update-all`,
		`ast-grep scan --update-all`,
		`ast-grep run -p foo -i`,
		`ast-grep run -p foo --interactive`,
		`sg run -p foo -Ui`,
		`sg group -c touch_x`,
	} {
		if got := FamilyOf(argvOf(cmd)); got != FamilyNone {
			t.Errorf("FamilyOf(%q) = %q, want none", cmd, got)
		}
	}
	for _, cmd := range []string{`ast-grep run -p foo`, `ast-grep scan`, `sg run -p foo -l go`, `ast-grep run -p foo -r bar`} {
		if got := FamilyOf(argvOf(cmd)); got != FamilyAstGrep {
			t.Errorf("FamilyOf(%q) = %q, want ast-grep", cmd, got)
		}
	}
}

// A raw chain segment that names an infrastructure or cloud program is not
// carried into the rewritten line (on Codex the rewrite is pre-approved).
func TestChainRefusesInfraSegments(t *testing.T) {
	const exe = "/opt/ds/downshift"
	for _, cmd := range []string{
		`git status && kubectl delete pod x`,
		`git status ; terraform apply`,
		`go test ./... && helm uninstall r`,
		`git status && psql -c drop`,
		`git status && gcloud compute ssh vm`,
		`git status && aws s3 rm s3://b`,
		`git status && docker rm -f c`,
		`git status && curl -X POST http://x/y`,
		`git status && reboot`,
		`git status && ssh host reboot`,
		`git status && chmod -R 777 x`,
		`git status && ast-grep run -p a -r b -U`,
	} {
		d := Decide(cmd, exe)
		if d.Rewrite {
			t.Errorf("Decide(%q) rewrote: %+v", cmd, d)
		}
	}
	// Plain chains still wrap.
	for _, cmd := range []string{`git status && go test ./...`, `cd app && go build ./...`} {
		if d := Decide(cmd, exe); !d.Rewrite {
			t.Errorf("Decide(%q) did not rewrite: %+v", cmd, d)
		}
	}
}

func TestPsqlRefusesEnvPrefix(t *testing.T) {
	for _, cmd := range []string{`PGSERVICE=x psql -c select_1`, `PGOPTIONS=-cdefault_transaction_read_only=off psql -c select_1`} {
		if d := Decide(cmd, "/opt/ds/downshift"); d.Rewrite {
			t.Errorf("Decide(%q) rewrote: %+v", cmd, d)
		}
	}
}
