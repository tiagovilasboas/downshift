// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"regexp"
)

// RiskSignalDef is one deterministic risk matcher. Risk signals do not vote
// for a complexity class; they set a floor. A task that touches secrets,
// auth, crypto, money, PII, destructive operations, or carries an
// instruction-override phrase must never be routed to the small tier on the
// strength of a single "rename"/"typo"/"format" keyword.
type RiskSignalDef struct {
	Category string
	Pattern  string
}

// RawRiskSignals is the canonical risk table. Patterns describe what the
// task touches (general security/data-safety vocabulary), not phrasings of
// any benchmark prompt. Money is matched on behaviour (charging, refunds,
// settlement, payment processing), not on data shapes that merely name it
// (a billingAddress field). To extend it, add a row and a table-driven case
// in risk_test.go.
var RawRiskSignals = []RiskSignalDef{
	// PT-BR terms follow the English ones. Go's \b is ASCII-only, so stems that
	// continue into an accented letter use \S* and carry no trailing \b.
	{"secret", `\b(secrets?|passwords?|passphrases?|credentials?|api[ _-]?keys?|private[ _-]?keys?|signing[ _-]?keys?|(access|refresh|bearer|session|auth|api|personal access) tokens?|ssh[ _-]?keys?|authorized[ _]keys|tokens? validation|jwts?|oauth\w*|saml|sso|2fa|mfa|totp|senhas?|credencia(l|is)|segredos?)\b|\btokens? de (acesso|refresh|bearer|sess\S*|autentica\S*|atualiza\S*|renova\S*)|\brotat\w*\s+(the\s+|all\s+)?(\w+\s+)?keys\b|\bchaves?\s+(d[ae]s?\s+apis?\b|api|privad\S*|secret\S*|de\s+assinatura|de\s+criptografia)|\brotacion\S*\s+(as\s+|a\s+)?chaves?|\bfrases?\s+secretas?\b|\btokens?\s+bearer\b|\bvalida\S{1,24}\s+d[eoa]s?\s+tokens?\b`},
	// Bare "auth" (the auth middleware, authMiddleware) is the same surface as
	// authentication or authorization, so a rename there is not mechanical.
	// Only as a standalone word: inside a path, file or hyphenated name
	// (auth.ts, auth/session.ts, auth-7d9f) it is a location, not the subject.
	{"authz", `(^|[^/.\w-])(auth|authn|authz)([^/.\w-]|$)|\b(permissions?|privileges?|permisos|is[ _]?admin|admin (role|rights|access)|rbac|acls?|authori[sz]ation|authenticat\w*|access control)\b|\b(autentica|autoriza)\S*|\bpermiss(ão|ao|ões|oes)|\bprivil\S*gios?\b|\bacceso a los datos\b|\bcontrole\s+de\s+acesso\b|\b(papel|perfil|direitos|acesso|fun[çc](?:[ãa]o|[õo]es))\s+de\s+(administrad\S*|admin\b)|\beh[ _]?admin(?:istrad\S*)?\b|(?:^|[^A-Za-z0-9_])é[ _]?admin(?:istrad\S*)?(?:[^A-Za-z0-9_]|$)`},
	// "signature" alone is not crypto ("function signature", "type signature",
	// "email signature"); only verification and signed-message contexts are.
	{"crypto", `\b(crypto\w*|encrypt\w*|decrypt\w*|hmac|tls|ssl|certificates?|(webhook|request|payload|message|digital|cryptographic|jwt|jws|hmac|rsa|ecdsa|ed25519|gpg|pgp) signatures?|signatures? (verification|validation|check|header|secret)s?|(verify|verifies|verifying|validate|validating|check|checking|skip|skipping) (the |a |an |its )?signatures?|signing (secrets?|certificates?))\b|\b((des|de|en)?criptograf|(en|de)cript)\S*|\bcertificad\S*\s+(digital|ssl|tls|x\.?509|raiz|autoassinad\S*|https|do servidor|expirad\S*)|\b(verifica\S*|valida\S*|checagem|renova\S*|rota\S*)\s+(d[oa]s?\s+|[oa]s?\s+|de\s+)?certificad\S*|\bcertifica\S*\s+digital|\bassinaturas?\s+(digitais|digital|criptogr\S*|hmac|jwt|jws|rsa|ecdsa|ed25519|gpg|pgp)\b|\bassinaturas?\s+d[eoa]s?\s+(webhooks?|payloads?|mensagens?|requisi\S*|jwts?|hmacs?)\b|\b(verifica\S*|valida\S*|checagem)\s+(de\s+|d[ao]s?\s+)assinaturas?\b|\b(pul\S*|desativ\S*|desabilit\S*)\s+(a\s+|as\s+)?verifica\S*\s+(de\s+|d[ao]s?\s+)assinaturas?\b|\bcabe[çc]alho\s+de\s+assinaturas?\b`},
	{"payment", `\b(payments? (processing|logic|flow|handler|service|gateway|capture|function|endpoint)|process(ing)? payments?|charg(e|es|ing) (the )?(customer|card|user)s?|double[- ]charg\w*|refunds?|settlements?|payouts?|stripe|pci|billing (logic|flow|cycle|calculation|engine)|pagamentos?|reembols\S*|estorn\S*)\b|\bcobr(ar|ando|ou|ad[oa]s?|an[çc]as?)\s+(o\s+|a\s+|do\s+|no\s+)?(cart\S*|clientes?)|\bcobr[ae]\s+(o|a|do|no)\s+(cart\S*|clientes?)|\b(l[óo]gica|fluxo|ciclo|c[áa]lculo|motor)\s+de\s+faturamento\b|\bcobr(?:ar|ando|ou|ad[oa]s?|an[çc]as?)\s+(duplicad\S*|em\s+dobro|duas\s+vezes)\b|\bdupla\s+cobran[çc]as?\b`},
	{"vuln", `\b(sql injection|injection|xss|csrf|ssrf|rce|sanitiz\w*|vulnerab\w*|cve-\d+|exploit\w*|security|seguran[çc]a|inje[çc](ão|ao|ões|oes)|cors|access-control-allow-[a-z]+|same[ _-]?site|(path|directory) traversal|zip slip|open redirects?|clickjacking|content[- ]security[- ]policy|(session|auth|authentication) cookies?|(run|runs|running|ran) as root|(public|publicly|world)[- ](readable |accessible |writable )?(s3 )?buckets?)\b|\btravessia\s+de\s+(diret\S*|caminho)|\bcookies?\s+de\s+(sess[ãa]o|autentica\S*)`},
	{"pii", `\b(pii|gdpr|lgpd|personal data|ssn|social security|credit cards?|dados pessoais)\b|\b(cpf|cnpj)s?\s+d[oa]s?\s+(clientes?|usu\S*rios?|titular\S*|pessoas?|funcion\S*rios?)|\bcart(ões|oes|ão|ao)\s+de\s+cr[ée]dito\b`},
	{"destructive", `rm\s+-rf|\bchmod\s+(-r\s+)?(0?777|a\+rwx)\b|\bworld[- ]writable\b|\bdrop\s+(the\s+)?(\w+\s+)?(table|database|schema|column)s?\b|\btruncate\s+(the\s+)?(\w+\s+)?tables?\b|\bdelete\s+from\s+\w+\s*(;|where\b|$)|\bdelete\s+(all|every)\b|\bforce[- ]push\b|push\s+(-f|--force)\b|reset\s+--hard|\bwipe\b|\bpurge\b|\bprod(uction)? (data|database|db)\b|\b(dados|banco( de dados)?|base( de dados)?) de produ[çc][ãa]o|\b(drop|exclu|apag|delet|trunc)\S*\s+(as?\s+|o\s+)?(tabela|coluna|banco de dados|base de dados|schema)s?\b|\b(apag|exclu|delet)\S*\s+(todos\s+)?(os\s+)?dados\b|\bpurga(r|ndo|do|da|dos|das)?\b|\bpurgue\b|\bpush\s+for[çc]ado\b|\b(apag|exclu|delet)\S*\s+tudo\b`},
	// Shared-state changes: a rename that also touches a lock, an atomic or a
	// race is not mechanical.
	{"concurrency", `\b(race conditions?|data races?|race detector|(fix|fixes|fixing|fixed|avoid\w*|prevent\w*|introduc\w*|caus\w*)\s+(the\s+|a\s+|this\s+)?races?|mutex(es)?|rwmutex|deadlocks?|atomic\.\w+|atomic (operations?|counters?|ops|loads?|stores?|swaps?|increments?)|atomics|sync\.(once|wait ?group|cond)|semaphores?|thread[- ]safe\w*)\b|\bcondi\S* de corrida|\b(opera\S*|contador(?:es)?|incrementos?|cargas?|trocas?|armazenamentos?)\s+at[ôo]mic[oa]s?\b|\bcorridas?\s+de\s+dados\b|\bdetector\s+de\s+(corridas?|races?)\b|\bsegur[oa]s?\s+para\s+threads?\b|\bexclus[ãa]o\s+m[úu]tua\b`},
	{"prompt_injection", `\b(ignore|disregard|forget|override)\s+(all\s+|any\s+|the\s+|your\s+)?(previous|prior|above|earlier|system)\s+(instructions?|prompts?|rules?)\b|\bsystem prompt\b|\bthis is (a |an )?(trivial|simple|easy|tiny)\b|\b(route|send|use)\b.{0,20}\b(cheap|cheapest|small|smallest|haiku|mini|nano)\s+(model|tier)\b|\b(ignore|desconsider\S*|esque[çc]\S*|sobrescrev\S*|anul\S*)\s+(todas\s+|todos\s+)?(d?[ao]s?\s+)?(instru[çc](ões|oes)|regras|prompts?)\s+(anteriores|pr[ée]vias?|acima|do\s+sistema)\b|\bprompts?\s+d[eo]\s+sistema\b|\b(esta|essa|isto|isso)\s+(é|e)\s+(uma\s+)?(tarefa\s+)?(trivial|simples|f[áa]cil|min[úu]scul[oa]s?)\b|\b(use|usar|usa|utilize|utiliza|utilizar|envie|enviar|mande|mandar|roteie|rotear)\b.{0,40}\b((mais\s+)?(barat\S*|pequen\S*|menor|min[úu]scul\S*|haiku|mini|nano)\s+(modelo|tier)|(modelo|tier)\s+(mais\s+)?(barat\S*|pequen\S*|menor|haiku|mini|nano))\b`},
}

// riskHighStakesWorkRE matches M3 parallel bucket prompts only (not generic
// "implement X" medium-risk tasks in the legacy benchmark splits).
var riskHighStakesWorkRE = regexp.MustCompile(`(?i)\bidempot\w*\b[^.\n]{0,60}\b(payments?|pagamentos?|reembolsos?|refunds?)|\b(payments?|pagamentos?|reembolsos?|refunds?)[^.\n]{0,60}\bidempot\w*\b|\bauthorization boundaries\b|\brevis\w*\s+as\s+permiss`)

// riskLiterals is a prefilter, not a second table. For each category it lists
// lowercase ASCII literals such that every match of that category's regex,
// run on splitIdentifiers(prompt) and then on foldRiskText, contains at least
// one of them. A text with none of them cannot match, so the regex is skipped.
// The lists are supersets of the required words (e.g. "auth" covers "authz"),
// so a missing entry only costs speed: a category without one always runs its
// regex. Adding a pattern means adding its required word here too.
var riskLiterals = map[string][]string{
	"secret": {"secret", "password", "passphrase", "credential", "api", "private", "signing",
		"token", "jwt", "oauth", "saml", "sso", "2fa", "mfa", "totp", "senha", "credencia",
		"segredo", "rotat", "chave", "ssh", "authorized"},
	"authz": {"auth", "permission", "privilege", "permisos", "admin", "rbac", "acl", "autentica",
		"autoriza", "access control", "permiss", "privil", "acceso a los datos", "acesso"},
	"crypto": {"crypt", "hmac", "tls", "ssl", "certifica", "signature", "signing", "criptograf",
		"assinatur", "decript", "encript"},
	"payment": {"payment", "charg", "refund", "settlement", "payout", "stripe", "pci",
		"billing", "pagamento", "reembols", "estorn", "cobr", "fatur"},
	"vuln": {"injection", "xss", "csrf", "ssrf", "rce", "sanitiz", "vulnerab", "cve-",
		"exploit", "security", "seguran", "inje", "cors", "access-control-allow", "samesite", "same site", "traversal", "zip slip", "redirect",
		"clickjack", "content security", "content-security", "cookie", "as root", "bucket", "travessia"},
	"pii": {"pii", "gdpr", "lgpd", "personal data", "ssn", "social security", "credit card",
		"cpf", "cnpj", "dados pessoais", "cart"},
	"destructive": {"-rf", "drop", "trunc", "delet", "force", "push", "reset", "wipe", "purge",
		"prod", "apag", "exclu", "purg", "chmod", "writable"},
	"concurrency": {"race", "mutex", "deadlock", "atomic", "sync.", "semaphore", "thread",
		"condi", "atôm", "corrida", "exclus"},
	"prompt_injection": {"ignore", "disregard", "forget", "override", "system prompt", "this is",
		"cheap", "small", "haiku", "mini", "nano", "desconsidere", "desconsider", "esque", "tarefa",
		"prompt", "trivial", "simples", "fácil", "facil", "minúsc", "minusc", "barat", "modelo",
		"pequen", "menor", "sobrescrev", "anul"},
}
