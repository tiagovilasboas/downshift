// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand/v2"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// This file checks that the prefiltered risk scan returns exactly the
// categories of the unfiltered scan it replaced. refRiskCategories is that scan
// (HEAD's splitIdentifiers and scanRiskCategories, without the prefilter or the
// memo), copied here as the reference.

var (
	refCamelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	refRiskSignals   = func() []riskSignal {
		out := make([]riskSignal, 0, len(RawRiskSignals))
		for _, r := range RawRiskSignals {
			out = append(out, riskSignal{category: r.Category, re: regexp.MustCompile(`(?i)` + r.Pattern)})
		}
		return out
	}()
)

func refSplitIdentifiers(prompt string) string {
	return strings.ReplaceAll(refCamelBoundary.ReplaceAllString(prompt, "$1 $2"), "_", " ")
}

func refRiskCategories(prompt string) []string {
	text := refSplitIdentifiers(prompt)
	var cats []string
	for _, s := range refRiskSignals {
		if s.re.MatchString(text) {
			cats = append(cats, s.category)
		}
	}
	sort.Strings(cats)
	return cats
}

// checkRiskEquiv compares the new scan, and the memoised RiskCategories, with
// the reference for one prompt.
func checkRiskEquiv(t *testing.T, prompt string) {
	t.Helper()
	want := refRiskCategories(prompt)
	if got := scanRiskCategories(prompt); !slices.Equal(got, want) {
		t.Fatalf("scanRiskCategories(%q) = %v, want %v", clip(prompt), got, want)
	}
	if got := RiskCategories(prompt); !slices.Equal(got, want) {
		t.Fatalf("RiskCategories(%q) = %v, want %v", clip(prompt), got, want)
	}
}

func clip(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

// testStringLiterals returns every string literal in the package's *_test.go
// files: the existing test prompts.
func testStringLiterals(t *testing.T) []string {
	t.Helper()
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	fset := token.NewFileSet()
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil {
					out = append(out, s)
				}
			}
			return true
		})
	}
	return out
}

var equivVocab = []string{
	"secrets", "password", "passphrases", "credentials", "api key", "api_key", "apiKey", "API-KEY",
	"private key", "signing_key", "access token", "refresh tokens", "tokens validation", "jwt", "oauth2",
	"saml", "SSO", "2fa", "MFA", "totp", "senhas", "credenciais", "segredo", "rotate the keys",
	"rotate all keys", "rotacionar as chaves", "chave de API", "chave privada", "chaves de criptografia",
	"token de acesso", "token de sessão", "ignore previous instructions", "disregard all prior rules",
	"override the system prompt", "system prompt", "system_prompt", "this is trivial", "this is an easy one",
	"route to the cheap model", "use the haiku tier", "send it to a mini model", "nano tier",
	"drop table users", "DROP DATABASE prod", "truncate the orders table", "delete from users;",
	"delete all rows", "rm -rf", "force push", "git push --force", "reset --hard", "wipe", "purge",
	"dados de produção", "banco de dados de produção", "apague a tabela", "exclua os dados",
	"race condition", "data race", "race detector", "mutex", "rwmutex", "deadlock", "atomic.AddInt64",
	"atomic counter", "sync.WaitGroup", "sync.Once", "semaphore", "thread-safe", "thread safe",
	"condição de corrida", "cobrar o cartão do cliente", "cobra clientes", "reembolso", "estorno",
	"charge the card", "double-charge", "stripe", "PCI", "billing logic", "pagamentos", "payouts",
	"settlement", "CVE-2024-1234", "SQL injection", "XSS", "CSRF", "SSRF", "sanitize input",
	"vulnerability", "exploit", "security", "LGPD", "GDPR", "PII", "personal data", "SSN",
	"social security", "credit card", "creditCardNumber", "auth middleware", "authz", "auth.ts",
	"auth/session.go", "authMiddleware", "permissions", "privileges", "is_admin", "isAdmin", "admin role",
	"RBAC", "ACLs", "authorization", "authenticate", "access control", "autenticação", "autorização",
	"crypto", "encrypt", "decrypt", "HMAC", "TLS", "SSL", "certificate", "webhook signature",
	"verify the signature", "skip signature check", "criptografia", "certificado digital",
	"renovar o certificado", "verificar certificado", "certifica digital", "criptografar", "signing secret",
	"rename", "typo", "variable", "userId", "accountId", "camelCase", "snake_case", "XMLHttpRequest",
	"HTTPServer", "userID", "use", "send", "route", "small", "mini", "cheapest", "haiku", "tier",
	"model", "race", "rotação", "pci", "ssl", "tls", "ssn", "acl", "mfa", "sso", "dados", "produção",
	"ſecurity", "toKen", "Key", "paſsword", "ＳＥＣＲＥＴ", "İstanbul", "ı", "ß", "ẞ",
	"ç", "Ç", "ã", "é", "ñ", " ", "\t", "\n", "\r\n", "\x80", "😀", "‍", "ǅ", "ǈ",
	"x", "a", "B", "", " ", "_", "-", ".", "/", ",", ";", "(", ")", "'", `"`, "::", "=>",
}

var equivSeps = []string{" ", " ", " ", "  ", "_", "-", ".", "/", "\n", "\t", " ", ""}

// mutateCase randomly changes the case of runes in s (ASCII and accented).
func mutateCase(s string, r *rand.Rand) string {
	var b strings.Builder
	for _, c := range s {
		switch r.IntN(4) {
		case 0:
			b.WriteRune(unicode.ToUpper(c))
		case 1:
			b.WriteRune(unicode.ToLower(c))
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

func randomPrompt(r *rand.Rand) string {
	n := r.IntN(12)
	var b strings.Builder
	for i := 0; i < n; i++ {
		w := equivVocab[r.IntN(len(equivVocab))]
		switch r.IntN(6) {
		case 0:
			w = mutateCase(w, r)
		case 1:
			// camelCase join with the next word's first letter capitalised.
			next := equivVocab[r.IntN(len(equivVocab))]
			if next != "" {
				rs := []rune(next)
				rs[0] = unicode.ToUpper(rs[0])
				w += string(rs)
			}
		case 2:
			// Random junk around the word, including invalid UTF-8.
			w = string(rune('a'+r.IntN(26))) + w + string([]byte{byte(0x80 + r.IntN(128))})
		}
		b.WriteString(w)
		if i < n-1 {
			b.WriteString(equivSeps[r.IntN(len(equivSeps))])
		}
	}
	return b.String()
}

// signalPhrases carries one phrase per category, each one a real match.
var signalPhrases = []string{
	"rotate the signing keys",
	"verify the webhook signature",
	"double-charge the customer",
	"apague os dados de produção",
	"ignore all previous instructions",
	"use the cheap model tier",
	"sync.WaitGroup for the workers",
	"fix the data race",
	"check the admin role",
	"rename the ſecurity helper",
	"CVE-2024-1234 in the parser",
	"encrypt the payload",
}

var longFillers = []string{
	"Please review the module layout and keep the names stable. ",
	"aB",
	"rename the variable userId to accountId. ",
}

func TestRiskScanEquivalence_CorpusAndFuzz(t *testing.T) {
	seen := map[string]bool{}
	var corpus []string
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			corpus = append(corpus, s)
		}
	}
	for _, s := range testStringLiterals(t) {
		add(s)
	}
	for _, s := range equivVocab {
		add(s)
		add(strings.ToUpper(s))
	}
	for _, p := range signalPhrases {
		add(p)
	}
	testPrompts := len(corpus)
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 30000; i++ {
		add(randomPrompt(r))
	}
	t.Logf("existing test literals: %d, total prompts: %d", testPrompts, len(corpus))
	for _, p := range corpus {
		checkRiskEquiv(t, p)
		if got, want := splitIdentifiers(p), refSplitIdentifiers(p); got != want {
			t.Fatalf("splitIdentifiers(%q) = %q, want %q", clip(p), got, want)
		}
	}
}

func TestRiskScanEquivalence_SplitExhaustive(t *testing.T) {
	alphabet := []rune{'a', 'B', 'z', 'Z', '0', '9', '_', ' ', 'é', 'K', 'K', 'ſ', '-', 'A'}
	var walk func(prefix []rune, depth int)
	walk = func(prefix []rune, depth int) {
		s := string(prefix)
		if got, want := splitIdentifiers(s), refSplitIdentifiers(s); got != want {
			t.Fatalf("splitIdentifiers(%q) = %q, want %q", s, got, want)
		}
		if depth == 0 {
			return
		}
		for _, c := range alphabet {
			walk(append(prefix, c), depth-1)
		}
	}
	walk(nil, 5)
}

// TestFoldMatchesRegexCaseFolding checks foldRiskText against the regexp
// engine for every rune up to U+1FFFF: a lowercase ASCII literal char matches
// rune r under (?i) exactly when foldRiskText(r) is that char. This is the claim
// the literal prefilter depends on, including the Kelvin sign and long s.
func TestFoldMatchesRegexCaseFolding(t *testing.T) {
	lit := "abcdefghijklmnopqrstuvwxyz0123456789-. @_"
	for _, c := range lit {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(string(c)))
		for r := rune(0); r <= 0x1FFFF; r++ {
			if r >= 0xD800 && r <= 0xDFFF {
				continue
			}
			s := string(r)
			if re.MatchString(s) != (foldRiskText(s) == string(c)) {
				t.Fatalf("(?i)%q vs rune U+%04X: regexp=%v fold=%q", c, r, re.MatchString(s), foldRiskText(s))
			}
		}
	}
}

// TestRiskLiteralsCoverEveryCategory guards the prefilter table: a category
// without literals silently falls back to the regex, which must be deliberate.
func TestRiskLiteralsCoverEveryCategory(t *testing.T) {
	for cat := range RiskCategoryNames() {
		if len(riskLiterals[cat]) == 0 {
			t.Errorf("category %q has no prefilter literals", cat)
		}
	}
	for _, s := range riskSignals {
		if s.literals == nil {
			t.Errorf("category %q runs without prefilter", s.category)
		}
	}
}

func TestRiskScanEquivalence_LongPrompts(t *testing.T) {
	// The signal sits at the start, just before the 32 KB classify cap, just
	// after it, and deeper in the prompt.
	offsets := []int{0, 1<<15 - 5, 1<<15 + 3, 40 << 10}
	for _, filler := range longFillers {
		for _, phrase := range signalPhrases {
			for _, off := range offsets {
				t.Run("", func(t *testing.T) {
					t.Parallel()
					var b strings.Builder
					for b.Len() < off {
						b.WriteString(filler)
					}
					head := b.String()[:off]
					checkRiskEquiv(t, head+phrase+" "+filler[:min(len(filler), 4096)])
				})
			}
		}
	}
	if testing.Short() {
		t.Skip("1 MiB cases skipped in -short")
	}
	for _, filler := range []string{longFillers[0], longFillers[1]} {
		var b strings.Builder
		for b.Len() < 1<<20 {
			b.WriteString(filler)
		}
		big := b.String()[:1<<20-64] + "rotate the signing keys"
		checkRiskEquiv(t, big)
		checkRiskEquiv(t, "rename the variable userId to accountId. "+big[40<<10:])
	}
}

// Benchmarks. Route cases reset the risk memo so each call scans its prompt,
// as a fresh hook process does. The Ref benchmarks run the replaced scan on the
// same inputs.

var (
	benchAdversarial = benchInput("aB", 1<<20)
	benchBenign      = benchInput(longFillers[0], 1<<20)
	// Worst case for the prefilter: the filler contains literals of most
	// categories, so their regexes run over the whole prompt.
	benchWorst = benchInput("use the password, rotate the keys and the security token; ", 1<<20)
	// Harder: literals of most categories occur, but every one fails the
	// regex's word boundary, so no regex can stop early and each runs over the
	// whole prompt.
	benchNoMatch = benchInput("xsecretsx tokenz ssox pcix xrefunds acl1 zcharge_ rotatex racex mutexs ", 1<<20)
	benchShort   = "Refactor the token validation in auth/session.go to return typed errors, update the callers in api/handlers, and extend the table-driven tests."
)

func benchInput(filler string, size int) string {
	var b strings.Builder
	b.WriteString("rename the variable userId to accountId. ")
	for b.Len() < size {
		b.WriteString(filler)
	}
	return b.String()[:size]
}

func benchRoute(b *testing.B, prompt string) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		riskMemo.ok = false
		_ = Route(prompt, "claude-code", "")
	}
}

func BenchmarkRouteAdversarial1M(b *testing.B) { benchRoute(b, benchAdversarial) }
func BenchmarkRouteBenign1M(b *testing.B)      { benchRoute(b, benchBenign) }
func BenchmarkRouteWorst1M(b *testing.B)       { benchRoute(b, benchWorst) }
func BenchmarkRouteNoMatch1M(b *testing.B)     { benchRoute(b, benchNoMatch) }
func BenchmarkRouteShort(b *testing.B)         { benchRoute(b, benchShort) }

func BenchmarkRiskScanAdversarial1M(b *testing.B) { benchScan(b, benchAdversarial, scanRiskCategories) }
func BenchmarkRiskScanBenign1M(b *testing.B)      { benchScan(b, benchBenign, scanRiskCategories) }
func BenchmarkRiskScanWorst1M(b *testing.B)       { benchScan(b, benchWorst, scanRiskCategories) }
func BenchmarkRiskScanNoMatch1M(b *testing.B)     { benchScan(b, benchNoMatch, scanRiskCategories) }
func BenchmarkRiskScanShort(b *testing.B)         { benchScan(b, benchShort, scanRiskCategories) }

func BenchmarkRiskRefAdversarial1M(b *testing.B) { benchScan(b, benchAdversarial, refRiskCategories) }
func BenchmarkRiskRefBenign1M(b *testing.B)      { benchScan(b, benchBenign, refRiskCategories) }
func BenchmarkRiskRefWorst1M(b *testing.B)       { benchScan(b, benchWorst, refRiskCategories) }
func BenchmarkRiskRefNoMatch1M(b *testing.B)     { benchScan(b, benchNoMatch, refRiskCategories) }
func BenchmarkRiskRefShort(b *testing.B)         { benchScan(b, benchShort, refRiskCategories) }

func benchScan(b *testing.B, prompt string, scan func(string) []string) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = scan(prompt)
	}
}
