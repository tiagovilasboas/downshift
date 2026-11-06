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

// mysqlValueOptions are the connection options that take a value, attached
// (--host=db, -hdb) or as the next word. mysqlFlagOptions take none. Any other
// option fails closed: --init-command and a second -e run SQL, --plugin-dir
// and --default-auth load code, --defaults-file reads options from any file,
// --tee writes one and --pager runs a program.
var (
	mysqlValueOptions = map[string]bool{
		"host": true, "port": true, "user": true, "socket": true, "protocol": true,
		"database": true, "login-path": true, "ssl-mode": true, "ssl-ca": true,
		"default-character-set": true, "connect-timeout": true,
		"h": true, "P": true, "u": true, "S": true, "D": true,
	}
	mysqlFlagOptions = map[string]bool{
		"batch": true, "skip-column-names": true, "column-names": true, "table": true,
		"silent": true, "raw": true, "vertical": true, "html": true, "xml": true,
		"no-auto-rehash": true, "compress": true, "verbose": true, "quick": true,
		"safe-updates": true, "unbuffered": true, "show-warnings": true,
		"B": true, "N": true, "t": true, "s": true, "r": true, "E": true, "H": true,
		"X": true, "A": true, "C": true, "v": true, "q": true, "n": true, "U": true,
	}
)

// mysqlFamily detects a read-only mysql invocation: exactly one inline
// statement (-e / --execute) that mysqlPlainRead accepts, options from the
// lists above, a password only attached (-psecret, --password=secret, since
// the client reads a separate word as the database) and at most one database
// operand. The hook's safe-byte gate keeps quotes and semicolons out, but the
// direct `downshift compact --` call takes argv from the caller's shell, so
// this parse is what holds there.
func mysqlFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "mysql" {
		return FamilyNone
	}
	var stmts []string
	operands := 0
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		var name, value string
		var attached, ok bool
		switch {
		case strings.HasPrefix(a, "--"):
			if name, value, attached = strings.Cut(a[2:], "="); len(name) < 2 {
				return FamilyNone // a one-letter name is a short option only
			}
		case len(a) > 1 && a[0] == '-':
			name, value, ok = mysqlShortCluster(a[1:])
			if !ok {
				return FamilyNone
			}
			attached = value != ""
		default:
			operands++
			continue
		}
		switch {
		case name == "password" || name == "p":
		case mysqlFlagOptions[name] && !attached:
		case name == "execute" || name == "e" || mysqlValueOptions[name]:
			if !attached {
				// A next word that looks like an option is refused rather than
				// guessed: whether the client takes it as the value or as the
				// next option, this parse must agree with it.
				if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
					return FamilyNone
				}
				i++
				value = argv[i]
			}
			if name == "execute" || name == "e" {
				stmts = append(stmts, value)
			}
		default:
			return FamilyNone
		}
	}
	if len(stmts) != 1 || operands > 1 || !mysqlPlainRead(stmts[0]) {
		return FamilyNone
	}
	return FamilyMySQLQuery
}

// mysqlShortCluster reads one short-option word without its dash: value-less
// flags first (-BN), then at most one option that takes the rest of the word
// as its value (-BNe, -hdb, -psecret). It returns the last option's name and
// its attached value, or ok=false for a letter the lists do not hold.
func mysqlShortCluster(s string) (name, value string, ok bool) {
	for i := 0; i < len(s); i++ {
		c := s[i : i+1]
		switch {
		case c == "e" || c == "p" || mysqlValueOptions[c]:
			return c, s[i+1:], true
		case !mysqlFlagOptions[c]:
			return "", "", false
		}
		name = c
	}
	return name, "", true
}

// mysqlPlainRead reports whether stmt is one read statement: it begins with a
// read verb, ends in at most one terminator (; or \G), and holds no second
// statement, no backslash client command (\! runs a shell), no control
// character (a new line starts a client command such as system or source), no
// INTO (OUTFILE and DUMPFILE write files) and no ANALYZE (EXPLAIN ANALYZE runs
// the statement it explains).
func mysqlPlainRead(stmt string) bool {
	s := strings.TrimSpace(stmt)
	s = strings.TrimSuffix(strings.TrimSuffix(s, ";"), `\G`)
	if strings.ContainsAny(s, ";\\") {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return false
		}
	}
	lower := strings.ToLower(s)
	verb := lower
	if end := strings.IndexFunc(lower, func(r rune) bool { return r < 'a' || r > 'z' }); end >= 0 {
		verb = lower[:end]
	}
	if !mysqlReadVerbs[verb] {
		return false
	}
	for _, w := range strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '$')
	}) {
		if w == "into" || w == "analyze" {
			return false
		}
	}
	return true
}

// redisValueFlags are the redis-cli flags that take the next word as a value;
// redisPlainFlags take none. Both lists are closed. Any other flag fails
// closed, because a flag the parser does not know may take a value, and that
// value would then be read as the command: `-i get flushall` runs flushall.
// --eval and --rdb touch files, --cluster, --replica and --pipe* send commands
// the keyword does not name, and --bigkeys, --lru-test and the like scan or
// load the server.
var (
	redisValueFlags = map[string]bool{
		"-h": true, "-p": true, "-a": true, "-n": true, "-u": true, "-s": true,
		"-t": true, "-i": true, "-r": true, "-d": true, "-D": true,
		"--user": true, "--pass": true, "--sni": true, "--cacert": true,
		"--cacertdir": true, "--cert": true, "--key": true,
		"--tls-ciphers": true, "--tls-ciphersuites": true,
	}
	redisPlainFlags = map[string]bool{
		"-c": true, "-e": true, "-x": true, "-3": true, "-2": true,
		"--tls": true, "--raw": true, "--no-raw": true, "--csv": true,
		"--no-auth-warning": true, "--askpass": true,
	}
)

// redisFamily detects a read-only redis-cli (or valkey-cli) invocation. The
// command keyword is the first positional word after the flags in the lists
// above, and it must be a read-only Redis command. A flag outside the lists,
// or a value flag with no value word, never matches, whatever the keyword says.
func redisFamily(argv []string) Family {
	if len(argv) < 2 || (argv[0] != "redis-cli" && argv[0] != "valkey-cli") {
		return FamilyNone
	}
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		switch {
		case redisPlainFlags[a]:
		case redisValueFlags[a]:
			// A next word that looks like a flag is refused rather than
			// guessed, as in the mysql parse.
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
				return FamilyNone
			}
			i++
		case strings.HasPrefix(a, "-"):
			return FamilyNone
		case redisReadCommands[strings.ToLower(a)]:
			return FamilyRedisRead
		default:
			return FamilyNone
		}
	}
	return FamilyNone
}
