// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"bytes"
	"regexp"
)

// RedactedMark is the placeholder used when redacting secret values in place.
const RedactedMark = "[REDACTED]"

const redactedLine = "[downshift ccr: line withheld, it looked like a secret]"

// reSecret flags a line that looks like it carries a credential: auth
// headers, cookies, credentials in a URL, JWTs, well-known key prefixes and
// assignments to secret/token/pass/auth/key names.
var reSecret = regexp.MustCompile(`(?i)(\bsecrets?\s*[=:]|_secret\s*[=:]|\btokens?\s*[=:]|_token\s*[=:]|password\s*[=:]|passwd\s*[=:]|\bapi[_-]?keys?\s*[=:]|\bprivate[_-]?keys?\s*[=:]|\baccess[_-]?keys?\s*[=:]|\baccess[_-]?key_?id\s*[=:]|aws_secret\w*\s*[=:]|bearer |\bauthorization\s*:|cookie\s*[=:]|PRIVATE KEY|://[^/\s:@]+:[^@\s]+@|\beyJ[\w-]{10,}\.|\bgh[pousr]_[A-Za-z0-9]|\bsk-[A-Za-z0-9]|\b(?:AKIA|ASIA)[0-9A-Z]{16}|[_.-]pass(wd|word)?\s*[=:]|\bpass(wd|word)?\s*=|_auth\s*[=:])`)

// rePEMBegin and rePEMEnd bound a PEM block; every line between them, the
// base64 body included, is withheld.
var (
	rePEMBegin = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]+-----`)
	rePEMEnd   = regexp.MustCompile(`-----END [A-Z0-9 ]+-----`)
)

// AWS STS regexes.
var (
	reSTSJSON            = regexp.MustCompile(`(?i)("(?:AccessKeyId|SecretAccessKey|SessionToken)"\s*:\s*)"(?:[^"\\]|\\.)*"`)
	reSTSCLIKeyValue     = regexp.MustCompile(`(?i)\b((?:AccessKeyId|SecretAccessKey|SessionToken)\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s\r\n]+)`)
	reSTSTable           = regexp.MustCompile(`(?i)(\|\s*(?:AccessKeyId|SecretAccessKey|SessionToken)\s*\|\s*)[^|\r\n]+?(\s*\|)`)
	reSTSCredentialsText = regexp.MustCompile(`(?i)^(CREDENTIALS\s+)(\S+)(\s+\S+\s+)(\S+)(\s+)(\S+)(.*)$`)
	reASIAKey            = regexp.MustCompile(`\bASIA[0-9A-Z]{16}\b`)
)

// AWS SSM parameter regexes.
var (
	reSSMBlockBegin     = regexp.MustCompile(`(?i)"Parameters?"\s*:\s*[{\[]`)
	reSSMJSONValue      = regexp.MustCompile(`(?i)("Value"\s*:\s*)"(?:[^"\\]|\\.)*"`)
	reSSMTableValue     = regexp.MustCompile(`(?i)(\|\s*Value\s*\|\s*)[^|\r\n]+?(\s*\|)`)
	reSSMTableSecure    = regexp.MustCompile(`(?i)(\|\s*Secure(?:String)?\s*\|\s*)[^|\r\n]+?(\s*\|)`)
	reSSMTextParameters = regexp.MustCompile(`(?i)^(PARAMETERS\s+.*?)(SecureString|String|StringList)(\s+\S+\s+)(\S+.*)$`)
	reSSMTextSingle     = regexp.MustCompile(`(?i)^(arn:aws:ssm:.*?)(SecureString|String|StringList)(\s+\S+\s+)(\S+.*)$`)
)

// AWS environment-like output regexes.
var (
	reAWSEnvQuotedDouble = regexp.MustCompile(`(?i)\b(export\s+)?(AWS_SESSION_TOKEN|AWS_SECRET_ACCESS_KEY|AWS_SECURITY_TOKEN|AWS_ACCESS_KEY_ID)\s*=\s*"[^"\r\n]*"`)
	reAWSEnvQuotedSingle = regexp.MustCompile(`(?i)\b(export\s+)?(AWS_SESSION_TOKEN|AWS_SECRET_ACCESS_KEY|AWS_SECURITY_TOKEN|AWS_ACCESS_KEY_ID)\s*=\s*'[^'\r\n]*'`)
	reAWSEnvUnquoted     = regexp.MustCompile(`(?i)\b(export\s+)?(AWS_SESSION_TOKEN|AWS_SECRET_ACCESS_KEY|AWS_SECURITY_TOKEN|AWS_ACCESS_KEY_ID)\s*=\s*[^\s\r\n]+`)
)

// Redact withholds whole lines that look like credentials or PEM blocks,
// and redacts AWS STS credentials, AWS SSM parameter values, and AWS
// environment variables in-place to [REDACTED].
func Redact(raw []byte) []byte {
	return redact(raw)
}

// redact withholds every line that looks like a secret and every line of a PEM
// block, from its BEGIN line through its END line. Specific structured credentials
// (AWS STS, AWS SSM parameters, and AWS environment-like outputs) are sanitized
// in-place to [REDACTED] preserving surrounding syntax and non-secret fields.
func redact(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	lines := bytes.Split(raw, []byte("\n"))
	inPEM := false
	inSSMBlock := false
	ssmBraceDepth := 0

	for i, l := range lines {
		if !inPEM && rePEMBegin.Match(l) {
			inPEM = true
		}
		if inPEM {
			if rePEMEnd.Match(l) {
				inPEM = false
			}
			lines[i] = []byte(redactedLine)
			continue
		}

		if reSSMBlockBegin.Match(l) {
			inSSMBlock = true
		}

		lineModified := false
		current := l

		if inSSMBlock {
			ssmBraceDepth += bytes.Count(current, []byte("{")) + bytes.Count(current, []byte("[")) -
				bytes.Count(current, []byte("}")) - bytes.Count(current, []byte("]"))
			if reSSMJSONValue.Match(current) {
				current = reSSMJSONValue.ReplaceAll(current, []byte(`${1}"`+RedactedMark+`"`))
			}
			if ssmBraceDepth <= 0 && (bytes.Contains(current, []byte("}")) || bytes.Contains(current, []byte("]"))) {
				inSSMBlock = false
				ssmBraceDepth = 0
			}
			lines[i] = current
			continue
		}

		if reSSMTableValue.Match(current) {
			current = reSSMTableValue.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${2}`))
			lineModified = true
		} else if reSSMTableSecure.Match(current) {
			current = reSSMTableSecure.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${2}`))
			lineModified = true
		} else if reSSMTextParameters.Match(current) {
			current = reSSMTextParameters.ReplaceAll(current, []byte(`${1}${2}${3}`+RedactedMark))
			lineModified = true
		} else if reSSMTextSingle.Match(current) {
			current = reSSMTextSingle.ReplaceAll(current, []byte(`${1}${2}${3}`+RedactedMark))
			lineModified = true
		}

		if reSTSJSON.Match(current) {
			current = reSTSJSON.ReplaceAll(current, []byte(`${1}"`+RedactedMark+`"`))
			lineModified = true
		}
		if reSTSCredentialsText.Match(current) {
			current = reSTSCredentialsText.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${3}`+RedactedMark+`${5}`+RedactedMark+`${7}`))
			lineModified = true
		}
		if reSTSTable.Match(current) {
			current = reSTSTable.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${2}`))
			lineModified = true
		}
		if reSTSCLIKeyValue.Match(current) {
			current = reSTSCLIKeyValue.ReplaceAll(current, []byte(`${1}`+RedactedMark))
			lineModified = true
		}

		if reAWSEnvQuotedDouble.Match(current) {
			current = reAWSEnvQuotedDouble.ReplaceAll(current, []byte(`${1}${2}="`+RedactedMark+`"`))
			lineModified = true
		} else if reAWSEnvQuotedSingle.Match(current) {
			current = reAWSEnvQuotedSingle.ReplaceAll(current, []byte(`${1}${2}='`+RedactedMark+`'`))
			lineModified = true
		} else if reAWSEnvUnquoted.Match(current) {
			current = reAWSEnvUnquoted.ReplaceAll(current, []byte(`${1}${2}=`+RedactedMark))
			lineModified = true
		}

		if reASIAKey.Match(current) {
			current = reASIAKey.ReplaceAll(current, []byte(RedactedMark))
			lineModified = true
		}

		if lineModified {
			lines[i] = current
			continue
		}

		if reSecret.Match(current) {
			lines[i] = []byte(redactedLine)
		}
	}

	return bytes.Join(lines, []byte("\n"))
}
