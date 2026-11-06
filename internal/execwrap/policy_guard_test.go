// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

// The wrapper makes three safety claims of its own: a recursive rm, a
// statement that is not a plain read, and a Redis write never reach a wrap.
// A refusal is FamilyNone, which the hook leaves unrewritten and the direct
// `downshift compact --` call refuses with 126. The direct call receives argv
// from the caller's shell, so quotes, semicolons and newlines can reach these
// checks there even though the hook's safe-byte gate keeps them out.

func TestRmRecursiveNeverWraps(t *testing.T) {
	for _, argv := range [][]string{
		{"rm", "-rf", "x"},
		{"rm", "-fr", "x"},
		{"rm", "--recursive", "x"},
		{"rm", "-r", "x"},
		{"rm", "-R", "x"},
		{"rm", "-Rf", "x"},
		{"rm", "-r", "-f", "x"},
		{"rm", "-f", "-R", "x"},
		{"rm", "-rfv", "x"},
		{"rm", "-vrf", "x"},
		{"rm", "-dR", "x"},
		{"rm", "x", "-r"},    // GNU rm reads options after operands
		{"rm", "--rec", "x"}, // GNU accepts an unambiguous prefix of --recursive
		{"rm", "--r", "x"},
		{"grm", "-R", "x"},
		{"rm", "--", "-r"}, // an operand named -r is refused too: failing closed is cheap
	} {
		if got := FamilyOf(argv); got != FamilyNone {
			t.Errorf("%q: family %q, want none (a recursive rm must not wrap)", argv, got)
		}
	}
	for _, argv := range [][]string{
		{"rm", "x"},
		{"rm", "-f", "x"},
		{"rm", "-v", "a", "b"},
		{"rm", "-i", "x"},
		{"rm", "-d", "empty"},
		{"rm", "--force", "x"},
		{"grm", "-f", "x"},
	} {
		if got := FamilyOf(argv); got != FamilyRm {
			t.Errorf("%q: family %q, want %q", argv, got, FamilyRm)
		}
	}
}

func TestRmRecursiveNeverRewrittenByTheHook(t *testing.T) {
	for _, cmd := range []string{"rm -R build", "rm -r -f build", "rm -Rf build", "cd app && rm -R build"} {
		if d := Decide(cmd, "/opt/ds/downshift"); d.Rewrite {
			t.Errorf("%q rewritten to %q, want it left alone", cmd, d.Command)
		}
	}
}

func TestMySQLReadOnlyGuard(t *testing.T) {
	for _, argv := range [][]string{
		{"mysql", "-e", "select 1"},
		{"mysql", "-e", "SELECT id, name FROM users WHERE id = 7"},
		{"mysql", "--execute=show databases"},
		{"mysql", "--execute", "show tables"},
		{"mysql", "-eshow tables"},
		{"mysql", "-h", "db.local", "-P", "3306", "-u", "app", "-psecret", "-D", "shop", "-e", "describe orders"},
		{"mysql", "--host=db.local", "--user=app", "--batch", "--skip-column-names", "-e", "explain select 1"},
		{"mysql", "--host", "db.local", "--port", "3306", "-e", "desc orders"},
		{"mysql", "-BN", "-e", "select 1;"}, // one trailing terminator
		{"mysql", "-e", `select 1\G`},
		{"mysql", "shop", "-e", "show tables"}, // the database as an operand
		{"mysql", "--login-path=ro", "-e", "select 1"},
		{"mysql", "-e", "select * from analyze_runs"}, // a name that only contains a refused word
		{"mysql", "-BNe", "select 1"},                 // value-less flags, then -e
		{"mysql", "-p", "shop", "-e", "select 1"},     // -p alone prompts; shop is the database
	} {
		if got := FamilyOf(argv); got != FamilyMySQLQuery {
			t.Errorf("%q: family %q, want %q", argv, got, FamilyMySQLQuery)
		}
	}
	for _, argv := range [][]string{
		{"mysql"},
		{"mysql", "shop"}, // no statement: an interactive session
		{"mysql", "-e"},   // statement missing
		{"mysql", "-e", "delete from users"},
		{"mysql", "-e", "with x as (select 1) delete from users"},
		{"mysql", "-e", "select 1; drop table users"},
		{"mysql", "-e", "select 1;drop table users;"},
		{"mysql", "-e", "select 1", "-e", "drop table users"}, // the client runs the last -e
		{"mysql", "-e", "select 1", "--execute=drop table users"},
		{"mysql", "--init-command=drop table users", "-e", "select 1"},
		{"mysql", "--plugin-dir=/tmp/p", "--default-auth=evil", "-e", "select 1"},
		{"mysql", "--defaults-file=/tmp/my.cnf", "-e", "select 1"},
		{"mysql", "--defaults-extra-file=/tmp/my.cnf", "-e", "select 1"},
		{"mysql", "--tee=/tmp/out", "-e", "select 1"},
		{"mysql", "--pager=sh", "-e", "select 1"},
		{"mysql", "-e", "select * from users into outfile '/tmp/u'"},
		{"mysql", "-e", "select 1 /*! into dumpfile '/tmp/u' */"},
		{"mysql", "-e", "explain analyze delete u from users u join tmp t on t.id = u.id"},
		{"mysql", "-e", `select 1 \! rm -rf /`}, // a client shell escape
		{"mysql", "-e", "select 1\nsystem rm -rf /"},
		{"mysql", "-e", "select 1\nsource /tmp/x.sql"},
		{"mysql", "shop", "other", "-e", "select 1"}, // two operands
		{"mysql", "-e", "/*select*/ delete from users"},
		{"mysql", "--loose-init-command=drop table users", "-e", "select 1"},
		{"mysql", "--e=select 1"}, // a one-letter long option
		{"mysql", "-Bx", "-e", "select 1"},
		{"mysql", "--host", "-e", "drop table users", "-e", "select 1"}, // an option word as a value
	} {
		if got := FamilyOf(argv); got != FamilyNone {
			t.Errorf("%q: family %q, want none (not a plain read)", argv, got)
		}
	}
}

func TestRedisReadOnlyGuard(t *testing.T) {
	for _, argv := range [][]string{
		{"redis-cli", "get", "k"},
		{"redis-cli", "-h", "cache", "-p", "6379", "GET", "k"},
		{"redis-cli", "-n", "2", "hgetall", "h"},
		{"redis-cli", "--user", "ro", "--pass", "x", "info"},
		{"redis-cli", "--no-auth-warning", "-a", "x", "dbsize"},
		{"valkey-cli", "ping"},
	} {
		if got := FamilyOf(argv); got != FamilyRedisRead {
			t.Errorf("%q: family %q, want %q", argv, got, FamilyRedisRead)
		}
	}
	for _, argv := range [][]string{
		{"redis-cli"},
		{"redis-cli", "set", "k", "v"},
		{"redis-cli", "flushall"},
		{"redis-cli", "-n", "0", "flushdb"},
		{"redis-cli", "-a", "get", "del", "k"}, // the password is not the command
		{"redis-cli", "eval", "return 1", "0"},
		{"redis-cli", "config", "set", "dir", "/tmp"},
		{"redis-cli", "--pipe", "get"}, // sends raw protocol read from stdin
		{"redis-cli", "--pipe-timeout", "5", "get", "k"},
		{"redis-cli", "--eval", "/tmp/x.lua"},
		{"redis-cli", "--rdb", "/tmp/dump.rdb"},
		{"redis-cli", "--cluster", "call", "127.0.0.1:7000", "flushall"},
	} {
		if got := FamilyOf(argv); got != FamilyNone {
			t.Errorf("%q: family %q, want none (not a read)", argv, got)
		}
	}
}
