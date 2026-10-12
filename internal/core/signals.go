// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// SignalDef is one uncompiled classifier signal: a regex pattern, a weight,
// and the complexity class it votes for. Keeping the table here — separate
// from the classifier logic in classifier.go — allows tests to inspect,
// validate, and extend the signal set without touching the scoring engine.
type SignalDef struct {
	Pattern string
	Weight  int
	Class   Complexity
}

// RawSignals is the canonical, ordered list of classifier signals.
// The classifier compiles these at startup via compileSignals().
//
// To add or tune a signal: edit this slice and add a table-driven test case
// in classifier_edge_test.go that proves the change.
//
// Order matters only for readability — the scorer tries every signal
// independently and sums weights per class.
var RawSignals = []SignalDef{

	// ── COMPLEX — architecture, cross-system, security, migrations ────────

	// One pattern per stem: "re-architect" used to match both an architect and
	// a re-architect row and score twice for a single mention.
	{`\b(re-?)?architect\w*\b`, 3, Complex},
	{`\bre-?write\b`, 2, Complex},
	{`\bre-?design\b`, 3, Complex},
	{`\bsecurity\s+(audit|review)\b`, 3, Complex},
	{`\bcode\s+review\b`, 3, Complex},
	{`\bcross-system\b`, 3, Complex},
	// One row for "distributed": a second row for "distributed lock",
	// "distributed cache" and similar phrases scored one mention twice. The
	// bare word still covers those phrases.
	{`\bdistributed\b`, 2, Complex},
	{`\b(etcd|raft|consensus protocol)\b`, 3, Complex},
	{`\brace\s+condition\b`, 3, Complex},
	{`\bdeadlock\b`, 3, Complex},
	{`\bmulti-(system|service|region|tenant)\b`, 3, Complex},
	{`\bentire\s+(codebase|system|platform)\b`, 3, Complex},
	{`\btrade-?offs?\b`, 2, Complex},
	{`\bdesign\s+doc\b`, 3, Complex},
	{`\brfc\b`, 3, Complex},
	{`\bthink\s+through\b`, 2, Complex},
	{`\b(service mesh|event sourcing|cqrs|chaos engineering|end-to-end encryption|data lineage|cross-cluster|memory leak|cascading failure|streaming pipeline|model serving|key management)\b`, 3, Complex},
	// Distributed-data vocabulary: partitioning, agreement and copying state
	// across nodes, and concurrent multi-user editing.
	// "replication" only: "replicate the bug" is a reproduction, not a design.
	// "sharding" only: "shard the test suite across runners" is CI config.
	{`\b(sharding|consensus|replication|real[- ]?time collaborat\w*|operational transform\w*)\b`, 3, Complex},
	// Designing or building a whole system rather than a part of one: the verb
	// takes a system-sized noun as its object ("design a real-time collaborative
	// editing system", "design and build a reconciliation engine", "build a
	// collaboration platform"). Up to three modifiers, no punctuation. "build"
	// alone is narrower: building a system, strategy or algorithm is often a
	// routine feature. One row, so "design and build an engine" scores once.
	{`\b((design|architect)\s+(and\s+(build|implement)\s+)?(an?\s+|the\s+)?([\w-]+\s+){0,3}?(system|engine|platform|protocol|strategy|algorithm)|build\s+(an?\s+|the\s+)?([\w-]+\s+){0,3}?(engine|platform))s?\b`, 2, Complex},
	{`\bwhy\s+.{0,40}(fail|crash|hang|break|regress)\w*`, 2, Complex},
	// Portuguese mirror of the COMPLEX vocabulary above. Without it, hard PT-BR
	// work stays on mid. Go's \b and \w are ASCII-only, so stems stop before
	// the first accented letter and carry no trailing \b.
	{`\b(re-?desenh|re-?arquitet|arquitet)\w*`, 3, Complex},
	// Migration, EN and PT-BR in one row: the stem covers migrate, migration,
	// migre, migração. A separate English row scored one mention twice.
	{`\bmigr[aeo]\w*`, 3, Complex},
	{`\b(auditoria|revis\S{1,3}o)\s+de\s+seguran`, 3, Complex},
	{`\bcondi\S{1,4}o\s+de\s+corrida\b`, 3, Complex},
	{`\bsem\s+downtime\b`, 3, Complex},
	{`\bsistemas?\s+distribu`, 3, Complex},
	{`\b(toda|todo)\s+(a\s+|o\s+)?(base\s+de\s+c\S+digo|sistema|plataforma)\b`, 3, Complex},
	// Algorithmic / interpreter specs (general CS vocabulary, not eval prompts).
	{`\b(recursive-descent|longest common subsequence|operator precedence|postfix operators?)\b`, 3, Complex},
	{`\bbacktracking\b`, 3, Complex},
	{`\bcron\b`, 3, Complex},
	{`\b(directed )?graph\b`, 2, Complex},
	{`\bbinary heap\b`, 2, Complex},
	{`\b(concurrent(?:ly)?|goroutines?)\b`, 2, Complex},
	{`\bfn\s+func\s*\(`, 3, Complex},
	{`\[[0-9]+\]\[[0-9]+\]`, 2, Complex},
	// Reviewed retry wording. These patterns are already part of the shipped
	// classifier and covered by TestClassify.
	{`\b(previous\s+attempt|tentativa\s+anterior)\b`, 3, Complex},
	{`\b(compilation\s+failed|falha\s+de\s+compilação|erro\s+de\s+compilação|failed\s+to\s+compile)\b`, 3, Complex},
	{`\b(subagent\s+failed|subagente\s+falhou|testes?\s+falharam|tests?\s+failed)\b`, 3, Complex},
	{`\b(fix\s+(the\s+)?(build|compilation)|corrigir\s+(o\s+)?build|corrigir\s+(a\s+)?compilação)\b`, 3, Complex},

	// ── MEDIUM — features across files, refactors with context ───────────

	{`\brefactor\w*\b`, 2, Medium},
	{`\bimplement\b`, 2, Medium},
	{`\bintegrat\w*\b`, 2, Medium},
	{`\bfeature\b`, 2, Medium},
	{`\bendpoint\b`, 1, Medium},
	{`\bmodule\b`, 1, Medium},
	{`\bacross\s+\w+\s+files\b`, 2, Medium},
	{`\bdebug\b`, 2, Medium},
	{`\brevis(?:ar|e|ão|oes|ões)\b`, 2, Medium},
	// Changing a live schema without downtime takes multi-step planning
	// (backfill, concurrent index builds, lock timeouts) even when the edit
	// itself is "add a column".
	{`\b(zero[\s-]downtime|no[\s-]downtime|without\s+(any\s+)?downtime|without\s+(locking|blocking)\s+(writes|reads|the\s+table)|online\s+schema\s+change)\b`, 3, Medium},

	// ── SIMPLE — one isolated change ─────────────────────────────────────

	// The member being added may carry an article, modifiers or its own name
	// first: "add a new field", "add an optional retries argument",
	// "add verbose boolean flag". Up to three words, no punctuation.
	{`\badd\s+([\w-]+\s+){0,3}?(field|param|parameter|flag|property|column|attribute|argument|method|function)s?\b`, 2, Simple},
	{`\bfix\s+(the\s+)?bug\b`, 2, Simple},
	{`\bwrite\s+(an?\s+)?(single\s+|unit\s+)*(function|tests?\b|helper)\b`, 2, Simple},
	// "explain" marks a simple request only when it leads the prompt ("explain
	// what X does"). Inside a long analysis ("... and explain the fix in terms
	// of happens-before") it is not evidence of a simple task.
	{`^\W*((can|could)\s+you\s+|please\s+)?explain\b`, 2, Simple},
	{`\bwhat\s+(is|does|are)\b`, 1, Simple},
	{`\bsingle\s+(file|function)\b`, 2, Simple},
	{`\b(rotating cube|three\.js scene|hello world|landing page|one file|one function|one component)\b`, 3, Simple},

	// ── TRIVIAL — mechanical ──────────────────────────────────────────────

	{`\brename\b`, 3, Trivial},
	{`\brenome\w*\b`, 3, Trivial},
	{`\bformat\b`, 3, Trivial},
	{`\bindent\b`, 3, Trivial},
	{`\blint\b`, 2, Trivial},
	{`\bprettier\b`, 3, Trivial},
	{`\beslint\b`, 3, Trivial},
	{`\bgit\s+(commit|push|pull|status|add|stash|log|diff|checkout|branch)\b`, 3, Trivial},
	{`\bfix\s+(a\s+)?typo\b`, 3, Trivial},
	{`\badd\s+(a\s+)?comment\b`, 3, Trivial},
	{`\bboilerplate\b`, 2, Trivial},
	{`\bmove\s+(the\s+)?file\b`, 3, Trivial},
	{`\bdelete\s+(the\s+)?file\b`, 3, Trivial},
	{`\bbump\s+.{0,15}version\b`, 3, Trivial},
	{`\bremove\s+(unused|dead)\b`, 2, Trivial},
	{`\blist(?:ar|e)?\b.{0,40}\barquivos?\b`, 3, Trivial},

	// ── TRIVIAL / SIMPLE — mechanical edits and commands, EN + PT-BR ─────
	// Mined from the development half of the 1000-prompt exploration set
	// (docs/contrib/prompt-corpus.md); the other half is the control.
	// Removing clutter: commented-out code, unused imports, leftovers.
	{`\b(delete|remove|apague|apaga|remova|exclua|tire)\b.{0,40}\b(commented[- ]out|leftover|trailing|debugger|console\.log|dead code|comentad\w*|n[aã]o usad\w*)`, 3, Trivial},
	// Changing one configuration value: verb + setting + "to"/"para" + value.
	{`\b(set|change|update|troque|troca|trocar|muda|mude|altere|atualize|ajuste|bump)\b.{0,40}\b(default|padr[aã]o|port|porta|timeout|ttl|title|t[ií]tulo|font-size|label|icon|[ií]cone|constant|constante|version|vers[aã]o|deployment target|minsdkversion|batch size|pin)\b.{0,30}\b(to|para|=)\s+\S`, 3, Trivial},
	{`\bbump\s+\S+\s+to\s+v?\d`, 3, Trivial},
	// Mechanical file hygiene.
	{`\b(chmod|shebang|trailing (whitespace|newline)|single quotes|aspas simples|barrel file)\b|\.gitignore\b`, 3, Trivial},
	// Running a tool and reporting the result.
	{`^\W*(run|rode|rodar|execute|executar)\b.{0,30}\b(npm|npx|yarn|pnpm|pytest|cargo|go test|make|jest|vitest|eslint|tsc|terraform fmt|mvn|gradle|the tests?|os testes?)\b.{0,45}$`, 3, Trivial},
	{`\b(cargo (fmt|clippy)|terraform fmt|npm (audit|outdated|ls))\b`, 3, Trivial},
	// Showing something that already exists.
	{`^\W*(list|show|print|mostre|liste|imprima|exiba)\b.{0,40}\b(files|logs?|commits?|versions|vers[oõ]es|packages|pacotes|pods|arquivos)\b`, 2, Trivial},
	// PT-BR counterparts of the leading "explain" (short prompts only: a long
	// one is an analysis) and a translation request. The run-a-tool signal above
	// is likewise limited to short prompts; "run X and summarise by root cause"
	// is not mechanical.
	{`^\W*(explica|explique|qual (a )?diferen[cç]a|o que [eé])\b.{0,60}$`, 2, Simple},
	{`\b(traduza|translate)\b`, 2, Simple},
}
