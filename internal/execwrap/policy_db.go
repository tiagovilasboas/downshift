// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Database — MySQL (read-only), Redis (read-only)
// ════════════════════════════════════════════════════════════════════════════

// Database family constants. Both families are read-only: a mutating query or
// command never reaches a wrap.
const (
	FamilyMySQLQuery Family = "mysql-query"
	FamilyRedisRead  Family = "redis-read"
)

// mysqlReadVerbs are the SQL statement verbs a wrapped mysql command may run.
// The verb is the first token of the statement passed to -e / --execute; a
// statement that starts with anything else (insert, update, delete, drop,
// alter, create, truncate, grant, call, set, ...) never matches.
var mysqlReadVerbs = map[string]bool{
	"select": true, "show": true, "describe": true, "desc": true, "explain": true,
}

// redisReadCommands are the read-only Redis commands a wrapped redis-cli call
// may run. Every write, admin or scripting command (set, del, flushall, eval,
// config, ...) is absent and never matches.
var redisReadCommands = map[string]bool{
	"get": true, "mget": true, "strlen": true, "exists": true, "type": true,
	"ttl": true, "pttl": true, "keys": true, "scan": true, "dbsize": true,
	"hget": true, "hmget": true, "hgetall": true, "hkeys": true, "hvals": true,
	"hlen": true, "hexists": true, "lrange": true, "llen": true, "lindex": true,
	"smembers": true, "scard": true, "sismember": true, "srandmember": true,
	"zrange": true, "zrevrange": true, "zrangebyscore": true, "zcard": true,
	"zscore": true, "zrank": true, "getrange": true, "info": true, "ping": true,
	"memory": true, "object": true,
}

// dbFamily detects read-only database client commands: mysql and redis-cli.
func dbFamily(argv []string) Family {
	if family := mysqlFamily(argv); family != FamilyNone {
		return family
	}
	if family := redisFamily(argv); family != FamilyNone {
		return family
	}
	return FamilyNone
}

// mysqlFamily detects a read-only mysql invocation. The query must be supplied
// inline with -e / --execute and start with a read verb. Because Decide's
// safe-byte gate already forbids quotes, semicolons, pipes and the like, a
// chained or injected second statement cannot reach this check; the verb test
// is a defence-in-depth guard, not the only barrier.
func mysqlFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "mysql" {
		return FamilyNone
	}
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		// Inline statement: -e <stmt> / --execute <stmt> / --execute=<stmt>.
		var stmt string
		switch {
		case a == "-e" || a == "--execute":
			if i+1 >= len(argv) {
				return FamilyNone
			}
			i++
			stmt = argv[i]
		case strings.HasPrefix(a, "--execute="):
			stmt = strings.TrimPrefix(a, "--execute=")
		case strings.HasPrefix(a, "-e") && len(a) > 2:
			stmt = a[2:]
		default:
			continue
		}
		verb := stmt
		if sp := strings.IndexByte(stmt, ' '); sp >= 0 {
			verb = stmt[:sp]
		}
		if mysqlReadVerbs[strings.ToLower(verb)] {
			return FamilyMySQLQuery
		}
		return FamilyNone
	}
	return FamilyNone
}

// redisFamily detects a read-only redis-cli invocation. The command keyword is
// the first positional word after any connection flags, and it must be a
// read-only Redis command.
func redisFamily(argv []string) Family {
	if len(argv) < 2 || (argv[0] != "redis-cli" && argv[0] != "valkey-cli") {
		return FamilyNone
	}
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		if strings.HasPrefix(a, "-") {
			// Skip a flag value that is not itself a flag.
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") && connectionFlagTakesValue(a) {
				i++
			}
			continue
		}
		if redisReadCommands[strings.ToLower(a)] {
			return FamilyRedisRead
		}
		return FamilyNone
	}
	return FamilyNone
}

// connectionFlagTakesValue reports whether a redis-cli connection flag consumes
// the next word as its value, so that word is not mistaken for the command.
func connectionFlagTakesValue(flag string) bool {
	switch flag {
	case "-h", "-p", "-a", "-n", "-u", "--user", "--pass", "-t":
		return true
	}
	return false
}
