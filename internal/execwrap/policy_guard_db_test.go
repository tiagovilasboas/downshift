// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

// The database guards fail closed: a redis-cli flag the parser does not list
// is refused (its value would otherwise be read as the command), and a
// database client never runs behind an environment assignment, which can point
// it at an option file or another server.

const guardExe = "/opt/ds/downshift"

func TestRedisUnknownFlagFailsClosed(t *testing.T) {
	for _, cmd := range []string{
		"redis-cli -i get flushall",
		"redis-cli -s get flushall",
		"redis-cli -d get flushall",
		"redis-cli --sni get flushall",
		"redis-cli --cacert get flushall",
		"redis-cli --eval get",
		"redis-cli --rdb get",
		"redis-cli --functions-rdb get",
		"redis-cli --lru-test 100",
		"redis-cli --bigkeys",
		"redis-cli --cluster call get",
		"redis-cli --replica",
		"redis-cli --pipe",
		"redis-cli --pipe-timeout 5 get k",
		"redis-cli -h",          // a value flag with no value
		"redis-cli -h host -p",  // a value flag at the end
		"redis-cli -a -x get k", // a value that looks like a flag
		"redis-cli FLUSHALL",    // upper case is still a write
		"redis-cli FlushDB",
		"redis-cli -n 2 FLUSHALL",
		"valkey-cli -i get flushall",
		"docker exec app redis-cli -i get flushall",
		"cd x && redis-cli -s get flushall",
	} {
		if d := Decide(cmd, guardExe); d.Rewrite {
			t.Errorf("%q was rewritten (family %q): %q", cmd, d.Family, d.Command)
		}
	}
}

func TestRedisReadsStillClassify(t *testing.T) {
	for _, cmd := range []string{
		"redis-cli -h host -p 6379 get k",
		"redis-cli --raw keys abc",
		"redis-cli -n 2 hgetall h",
		"redis-cli -i 1 -r 3 get k",
		"redis-cli --tls --cacert ca.pem --sni host get k",
		"valkey-cli -c -3 mget a b",
	} {
		if d := Decide(cmd, guardExe); !d.Rewrite || d.Family != FamilyRedisRead {
			t.Errorf("%q: rewrite=%v family=%q, want a redis-read rewrite", cmd, d.Rewrite, d.Family)
		}
	}
	// The direct call can carry a glob in argv; the hook's safe-byte gate cannot.
	if got := FamilyOf([]string{"redis-cli", "--raw", "keys", "a*"}); got != FamilyRedisRead {
		t.Errorf("--raw keys a*: family %q, want %q", got, FamilyRedisRead)
	}
}

func TestDBClientRefusesEnvPrefix(t *testing.T) {
	for _, cmd := range []string{
		"HOME=/tmp/x mysql -e select",
		"MYSQL_HOME=/tmp/x mysql -e select",
		"MYSQL_TEST_LOGIN_FILE=/tmp/x/l mysql --login-path=ro -e select",
		"MYSQL_UNIX_PORT=/tmp/x.sock mysql -e select",
		"MYSQL_PWD=x mysql -e select",
		"REDISCLI_AUTH=x redis-cli get k",
		"FOO=1 redis-cli get k",
		"FOO=1 valkey-cli get k",
		"cd x && HOME=/tmp/x mysql -e select",
	} {
		if d := Decide(cmd, guardExe); d.Rewrite {
			t.Errorf("%q was rewritten (family %q): %q", cmd, d.Family, d.Command)
		}
	}
}

func TestEnvPrefixStillWrapsOtherFamilies(t *testing.T) {
	if d := Decide("FOO=1 go test ./...", guardExe); !d.Rewrite {
		t.Errorf("FOO=1 go test was refused: %+v", d)
	}
	for _, cmd := range []string{"mysql -e select", "redis-cli get k"} {
		if d := Decide(cmd, guardExe); !d.Rewrite {
			t.Errorf("%q was refused: %+v", cmd, d)
		}
	}
}
