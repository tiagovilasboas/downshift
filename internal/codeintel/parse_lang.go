// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"regexp"
	"sort"
	"strings"
)

var (
	// TypeScript / JavaScript regexes
	reTSJSFunc      = regexp.MustCompile(`^(?:export\s+(?:default\s+)?)?(?:async\s+)?function(?:\s*\*|\s+)\s*([A-Za-z0-9_$]+)`)
	reTSJSClass     = regexp.MustCompile(`^(?:export\s+(?:default\s+)?)?(?:abstract\s+)?class\s+([A-Za-z0-9_$]+)`)
	reTSJSInterface = regexp.MustCompile(`^(?:export\s+(?:default\s+)?)?interface\s+([A-Za-z0-9_$]+)`)
	reTSJSType      = regexp.MustCompile(`^(?:export\s+)?type\s+([A-Za-z0-9_$]+)(?:<[^>]*>)?\s*=`)
	reTSJSEnum      = regexp.MustCompile(`^(?:export\s+)?(?:const\s+)?enum\s+([A-Za-z0-9_$]+)`)
	reTSJSConst     = regexp.MustCompile(`^(?:export\s+)?(const|let|var)\s+([A-Za-z0-9_$]+)\s*(?::\s*[^=]+)?\s*=\s*(.*)`)
	reTSJSMethod    = regexp.MustCompile(`^(?:(?:public|private|protected|static|readonly|override|abstract|async)\s+)*(?:(?:get|set)\s+)?([A-Za-z0-9_$]+)\s*(?:<[^>]*>)?\s*\(`)
	reTSJSPropArrow = regexp.MustCompile(`^(?:(?:public|private|protected|static|readonly)\s+)*([A-Za-z0-9_$]+)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z0-9_$]+)?\s*=>`)

	// Python regexes
	rePythonClass = regexp.MustCompile(`^class\s+([A-Za-z0-9_]+)`)
	rePythonDef   = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z0-9_]+)`)

	// Rust regexes
	reRustFn     = regexp.MustCompile(`^(?:pub(?:\([^)]+\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern(?:\s+"[^"]+")?\s+)?fn\s+([A-Za-z0-9_]+)`)
	reRustStruct = regexp.MustCompile(`^(?:pub(?:\([^)]+\))?\s+)?struct\s+([A-Za-z0-9_]+)`)
	reRustEnum   = regexp.MustCompile(`^(?:pub(?:\([^)]+\))?\s+)?enum\s+([A-Za-z0-9_]+)`)
	reRustTrait  = regexp.MustCompile(`^(?:pub(?:\([^)]+\))?\s+)?trait\s+([A-Za-z0-9_]+)`)
	reRustImpl   = regexp.MustCompile(`^impl(?:<[^>]*>)?\s+(?:[A-Za-z0-9_]+\s+for\s+)?([A-Za-z0-9_]+)`)

	// PHP regexes
	rePHPClass     = regexp.MustCompile(`^(?:abstract\s+|final\s+)?class\s+([A-Za-z0-9_]+)`)
	rePHPInterface = regexp.MustCompile(`^interface\s+([A-Za-z0-9_]+)`)
	rePHPTrait     = regexp.MustCompile(`^trait\s+([A-Za-z0-9_]+)`)
	rePHPMethod    = regexp.MustCompile(`^(?:(?:public|protected|private|static|final|abstract)\s+)+function\s+([A-Za-z0-9_]+)\s*\(`)
	rePHPFunc      = regexp.MustCompile(`^function\s+([A-Za-z0-9_]+)\s*\(`)

	// Java regexes
	reJavaClass     = regexp.MustCompile(`^(?:(?:public|protected|private|abstract|static|final)\s+)*class\s+([A-Za-z0-9_]+)`)
	reJavaInterface = regexp.MustCompile(`^(?:(?:public|protected|private|abstract|static)\s+)*interface\s+([A-Za-z0-9_]+)`)
	reJavaEnum      = regexp.MustCompile(`^(?:(?:public|protected|private|static)\s+)*enum\s+([A-Za-z0-9_]+)`)
	reJavaRecord    = regexp.MustCompile(`^(?:(?:public|protected|private|static|final)\s+)*record\s+([A-Za-z0-9_]+)`)
	reJavaMethod    = regexp.MustCompile(`^(?:(?:public|protected|private|static|final|abstract|synchronized|native|default)\s+)+(?:<[^>]*>\s+)?(?:[A-Za-z0-9_<>\[\],\s]+)\s+([A-Za-z0-9_]+)\s*\(`)

	// C# regexes
	reCSClass     = regexp.MustCompile(`^(?:(?:public|protected|private|internal|abstract|static|sealed|partial)\s+)*class\s+([A-Za-z0-9_]+)`)
	reCSStruct    = regexp.MustCompile(`^(?:(?:public|protected|private|internal|readonly|partial)\s+)*struct\s+([A-Za-z0-9_]+)`)
	reCSInterface = regexp.MustCompile(`^(?:(?:public|protected|private|internal|partial)\s+)*interface\s+([A-Za-z0-9_]+)`)
	reCSEnum      = regexp.MustCompile(`^(?:(?:public|protected|private|internal)\s+)*enum\s+([A-Za-z0-9_]+)`)
	reCSMethod    = regexp.MustCompile(`^(?:(?:public|protected|private|internal|static|virtual|override|abstract|async|sealed|partial)\s+)+(?:[A-Za-z0-9_<>\[\],\s\?]+)\s+([A-Za-z0-9_]+)\s*(?:<[^>]*>)?\s*\(`)

	// Import extraction regexes
	reTSJSImport     = regexp.MustCompile(`(?:import\s+(?:(?:[A-Za-z0-9_{}\s*,*]+)\s+from\s+)?["']([^"']+)["']|require\s*\(\s*["']([^"']+)["']\s*\))`)
	rePythonImport   = regexp.MustCompile(`^\s*(?:from\s+([A-Za-z0-9_.]+)\s+import|import\s+([A-Za-z0-9_.,\s]+))`)
	reRustUse        = regexp.MustCompile(`^\s*(?:pub\s+)?use\s+([A-Za-z0-9_:{}*,\s]+);`)
	rePHPUse         = regexp.MustCompile(`^\s*use\s+([A-Za-z0-9_\\]+)(?:\s+as\s+[A-Za-z0-9_]+)?;`)
	reJavaImport     = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([A-Za-z0-9_.*]+);`)
	reCSUsing        = regexp.MustCompile(`^\s*using\s+(?:static\s+)?(?:[A-Za-z0-9_]+\s*=\s*)?([A-Za-z0-9_.]+);`)

	// Lightweight call regex
	reCallIdentifier = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

	tsExcludedMethodNames = map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"function": true, "return": true, "throw": true, "new": true,
		"import": true, "export": true, "class": true, "interface": true,
		"type": true, "const": true, "let": true, "var": true, "from": true,
	}

	cstyleExcludedMethodNames = map[string]bool{
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"return": true, "throw": true, "new": true, "class": true,
		"interface": true, "struct": true, "enum": true,
	}

	langCallExclusions = map[string]bool{
		// Common keywords & control flow
		"if": true, "for": true, "while": true, "switch": true, "catch": true,
		"return": true, "throw": true, "new": true, "sizeof": true, "typeof": true,
		"instanceof": true, "function": true, "def": true, "fn": true, "class": true,
		"import": true, "export": true, "require": true, "from": true, "as": true,
		"yield": true, "await": true, "async": true, "try": true, "finally": true,
		"else": true, "elif": true, "case": true, "default": true, "break": true,
		"continue": true, "with": true, "assert": true, "lambda": true, "print": true,
		// Builtins across JS/TS/Py/Rust/PHP/Java/C#
		"console": true, "log": true, "warn": true, "error": true, "info": true,
		"len": true, "range": true, "str": true, "int": true, "float": true,
		"bool": true, "list": true, "dict": true, "set": true, "tuple": true,
		"type": true, "super": true, "isinstance": true, "issubclass": true,
		"open": true, "sum": true, "min": true, "max": true, "map": true,
		"filter": true, "zip": true, "enumerate": true, "any": true, "all": true,
		"abs": true, "round": true, "pow": true, "repr": true, "iter": true,
		"next": true, "input": true, "format": true, "id": true, "hash": true,
		"println": true, "panic": true, "unwrap": true, "expect": true, "clone": true,
		"echo": true, "isset": true, "empty": true, "array": true, "var_dump": true,
		"die": true, "exit": true, "count": true, "in_array": true, "explode": true,
		"implode": true, "strlen": true, "substr": true, "trim": true,
		"System": true, "out": true, "Console": true, "WriteLine": true,
		"Write": true, "ReadLine": true, "ToString": true, "Equals": true,
		"GetHashCode": true, "GetType": true, "main": true,
	}
)

type classScope struct {
	name    string
	endLine int
}

// computeLineOffsets maps each 0-indexed line to its byte offset in src.
func computeLineOffsets(src string) []int {
	offsets := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// offsetToLine converts a byte offset to a 1-indexed line number.
func offsetToLine(lineOffsets []int, offset int) int {
	idx := sort.Search(len(lineOffsets), func(i int) bool {
		return lineOffsets[i] > offset
	})
	if idx == 0 {
		return 1
	}
	return idx
}

// maskStringsAndComments replaces the contents of string literals and comments with spaces,
// preserving all newlines and character positions so line numbers and offsets remain intact.
func maskStringsAndComments(src string, lang string) string {
	b := []byte(src)
	n := len(b)
	i := 0

	inLineComment := false
	inBlockComment := false
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inTripleDouble := false
	inTripleSingle := false

	isPython := lang == "py"
	isPHP := lang == "php"
	isJS := lang == "js" || lang == "ts"
	isRust := lang == "rs"

	for i < n {
		c := b[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			} else {
				b[i] = ' '
			}
			i++
			continue
		}

		if inBlockComment {
			if c == '*' && i+1 < n && b[i+1] == '/' {
				b[i] = ' '
				b[i+1] = ' '
				i += 2
				inBlockComment = false
				continue
			}
			if c != '\n' {
				b[i] = ' '
			}
			i++
			continue
		}

		if inTripleDouble {
			if c == '"' && i+2 < n && b[i+1] == '"' && b[i+2] == '"' {
				b[i] = ' '
				b[i+1] = ' '
				b[i+2] = ' '
				i += 3
				inTripleDouble = false
				continue
			}
			if c == '\\' && i+1 < n {
				b[i] = ' '
				if b[i+1] != '\n' {
					b[i+1] = ' '
				}
				i += 2
				continue
			}
			if c != '\n' {
				b[i] = ' '
			}
			i++
			continue
		}

		if inTripleSingle {
			if c == '\'' && i+2 < n && b[i+1] == '\'' && b[i+2] == '\'' {
				b[i] = ' '
				b[i+1] = ' '
				b[i+2] = ' '
				i += 3
				inTripleSingle = false
				continue
			}
			if c == '\\' && i+1 < n {
				b[i] = ' '
				if b[i+1] != '\n' {
					b[i+1] = ' '
				}
				i += 2
				continue
			}
			if c != '\n' {
				b[i] = ' '
			}
			i++
			continue
		}

		if inSingleQuote {
			if c == '\\' && i+1 < n {
				b[i] = ' '
				if b[i+1] != '\n' {
					b[i+1] = ' '
				}
				i += 2
				continue
			}
			if c == '\'' {
				b[i] = ' '
				inSingleQuote = false
				i++
				continue
			}
			if c != '\n' {
				b[i] = ' '
			}
			i++
			continue
		}

		if inDoubleQuote {
			if c == '\\' && i+1 < n {
				b[i] = ' '
				if b[i+1] != '\n' {
					b[i+1] = ' '
				}
				i += 2
				continue
			}
			if c == '"' {
				b[i] = ' '
				inDoubleQuote = false
				i++
				continue
			}
			if c != '\n' {
				b[i] = ' '
			}
			i++
			continue
		}

		if inBacktick {
			if c == '\\' && i+1 < n {
				b[i] = ' '
				if b[i+1] != '\n' {
					b[i+1] = ' '
				}
				i += 2
				continue
			}
			if c == '`' {
				b[i] = ' '
				inBacktick = false
				i++
				continue
			}
			if c != '\n' {
				b[i] = ' '
			}
			i++
			continue
		}

		// Comment triggers
		if !isPython && c == '/' && i+1 < n && b[i+1] == '/' {
			b[i] = ' '
			b[i+1] = ' '
			inLineComment = true
			i += 2
			continue
		}

		if (isPython || isPHP) && c == '#' {
			b[i] = ' '
			inLineComment = true
			i++
			continue
		}

		if !isPython && c == '/' && i+1 < n && b[i+1] == '*' {
			b[i] = ' '
			b[i+1] = ' '
			inBlockComment = true
			i += 2
			continue
		}

		// String triggers
		if isPython && c == '"' && i+2 < n && b[i+1] == '"' && b[i+2] == '"' {
			b[i] = ' '
			b[i+1] = ' '
			b[i+2] = ' '
			inTripleDouble = true
			i += 3
			continue
		}
		if isPython && c == '\'' && i+2 < n && b[i+1] == '\'' && b[i+2] == '\'' {
			b[i] = ' '
			b[i+1] = ' '
			b[i+2] = ' '
			inTripleSingle = true
			i += 3
			continue
		}

		if c == '"' {
			b[i] = ' '
			inDoubleQuote = true
			i++
			continue
		}

		if c == '\'' {
			if isRust {
				if i+2 < n && b[i+2] == '\'' {
					b[i] = ' '
					b[i+1] = ' '
					b[i+2] = ' '
					i += 3
					continue
				} else if i+3 < n && b[i+1] == '\\' && b[i+3] == '\'' {
					b[i] = ' '
					b[i+1] = ' '
					b[i+2] = ' '
					b[i+3] = ' '
					i += 4
					continue
				}
				i++
				continue
			}
			b[i] = ' '
			inSingleQuote = true
			i++
			continue
		}

		if isJS && c == '`' {
			b[i] = ' '
			inBacktick = true
			i++
			continue
		}

		i++
	}

	return string(b)
}

// findBodyEndLine finds the EndLine for a declaration starting at declOffset.
func findBodyEndLine(masked string, lineOffsets []int, startLine int, declOffset int, hasParams bool, isArrow bool) int {
	totalLines := len(lineOffsets)
	n := len(masked)
	i := declOffset

	if isArrow {
		arrowIdx := strings.Index(masked[i:], "=>")
		if arrowIdx != -1 {
			i += arrowIdx + 2
			for i < n && (masked[i] == ' ' || masked[i] == '\t') {
				i++
			}
			if i < n && masked[i] != '{' {
				startL := offsetToLine(lineOffsets, declOffset)
				endL := startL
				for i < n {
					if masked[i] == ';' {
						return offsetToLine(lineOffsets, i)
					}
					if masked[i] == '\n' {
						endL = offsetToLine(lineOffsets, i)
						break
					}
					i++
				}
				return endL
			}
		}
	} else if hasParams {
		firstParen := strings.IndexByte(masked[i:], '(')
		if firstParen != -1 {
			i += firstParen
			parenDepth := 0
			for i < n {
				if masked[i] == '(' {
					parenDepth++
				} else if masked[i] == ')' {
					parenDepth--
					if parenDepth == 0 {
						i++
						break
					}
				}
				i++
			}
		}
	}

	// Look for '{' or ';' after parameters/header
	braceStart := -1
	for i < n {
		c := masked[i]
		if c == '{' {
			braceStart = i
			break
		}
		if c == ';' {
			line := offsetToLine(lineOffsets, i)
			if line < startLine {
				return startLine
			}
			return line
		}
		i++
	}

	if braceStart == -1 {
		return startLine
	}

	braceDepth := 0
	for j := braceStart; j < n; j++ {
		c := masked[j]
		if c == '{' {
			braceDepth++
		} else if c == '}' {
			braceDepth--
			if braceDepth == 0 {
				line := offsetToLine(lineOffsets, j)
				if line-startLine > 5000 {
					return startLine + 5000
				}
				return line
			}
		}
	}

	if totalLines-startLine > 5000 {
		return startLine + 5000
	}
	return totalLines
}

// extractCallsFromSpan extracts up to 16 outgoing calls within a 1-indexed line span [startLine, endLine].
// Calls matching langCallExclusions or keywords are omitted.
func extractCallsFromSpan(masked string, lineOffsets []int, startLine, endLine int) []string {
	if startLine < 1 || endLine < startLine || len(lineOffsets) == 0 {
		return nil
	}

	startOffset := 0
	if startLine-1 < len(lineOffsets) {
		startOffset = lineOffsets[startLine-1]
	}

	endOffset := len(masked)
	if endLine < len(lineOffsets) {
		endOffset = lineOffsets[endLine]
	}
	if endOffset > len(masked) {
		endOffset = len(masked)
	}
	if startOffset >= endOffset {
		return nil
	}

	span := masked[startOffset:endOffset]
	matches := reCallIdentifier.FindAllStringSubmatch(span, -1)
	if len(matches) == 0 {
		return nil
	}

	var calls []string
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if name == "" || langCallExclusions[name] {
			continue
		}
		if !seen[name] {
			seen[name] = true
			calls = append(calls, name)
			if len(calls) >= 16 {
				break
			}
		}
	}

	return calls
}

// parseTSJSSymbols extracts structural symbols, calls, and imports from TypeScript and JavaScript files.
func parseTSJSSymbols(relPath, content string) ([]Symbol, []string) {
	masked := maskStringsAndComments(content, "ts")
	lineOffsets := computeLineOffsets(content)
	lines := strings.Split(content, "\n")
	maskedLines := strings.Split(masked, "\n")

	var symbols []Symbol
	var imports []string
	seenImports := make(map[string]bool)

	// Extract TS/JS imports
	for _, line := range lines {
		if len(imports) >= 32 {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if m := reTSJSImport.FindStringSubmatch(trimmed); len(m) > 1 {
			target := m[1]
			if target == "" && len(m) > 2 {
				target = m[2]
			}
			target = strings.TrimSpace(target)
			if target != "" && !seenImports[target] {
				seenImports[target] = true
				imports = append(imports, target)
				if len(imports) >= 32 {
					break
				}
			}
		}
	}

	var classStack []classScope

	for i := 0; i < len(lines); i++ {
		lineNum := i + 1
		maskedLine := maskedLines[i]
		trimmed := strings.TrimSpace(maskedLine)
		if trimmed == "" {
			continue
		}

		// Pop classes that have ended
		for len(classStack) > 0 && lineNum > classStack[len(classStack)-1].endLine {
			classStack = classStack[:len(classStack)-1]
		}

		colOffset := strings.Index(maskedLine, trimmed)
		declOffset := lineOffsets[i] + colOffset

		// 1. Class
		if m := reTSJSClass.FindStringSubmatch(trimmed); len(m) > 1 {
			name := m[1]
			endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         KindStruct,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      endLine,
			})
			classStack = append(classStack, classScope{name: name, endLine: endLine})
			continue
		}

		// 2. Interface
		if m := reTSJSInterface.FindStringSubmatch(trimmed); len(m) > 1 {
			name := m[1]
			endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         KindType, // TS interfaces map to KindType for DCI indexing compatibility
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      endLine,
			})
			continue
		}

		// 3. Type
		if m := reTSJSType.FindStringSubmatch(trimmed); len(m) > 1 {
			name := m[1]
			endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         KindType,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      endLine,
			})
			continue
		}

		// 4. Enum
		if m := reTSJSEnum.FindStringSubmatch(trimmed); len(m) > 1 {
			name := m[1]
			endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         KindType,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      endLine,
			})
			continue
		}

		// 5. Function
		if m := reTSJSFunc.FindStringSubmatch(trimmed); len(m) > 1 {
			name := m[1]
			endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, false)
			calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         KindFunction,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      endLine,
				Calls:        calls,
			})
			continue
		}

		// 6. Const / Let / Var / Arrow function
		if m := reTSJSConst.FindStringSubmatch(trimmed); len(m) > 2 {
			varKind := m[1]
			name := m[2]
			rhs := m[3]

			isArrow := false
			isFunc := false

			if strings.Contains(rhs, "=>") {
				isArrow = true
				isFunc = true
			} else if strings.Contains(rhs, "function") || strings.Contains(rhs, "async") {
				isFunc = true
			} else {
				// Check multi-line arrow
				nextSemi := strings.IndexByte(masked[declOffset:], ';')
				nextArrow := strings.Index(masked[declOffset:], "=>")
				if nextArrow != -1 && (nextSemi == -1 || nextArrow < nextSemi) {
					isArrow = true
					isFunc = true
				}
			}

			kind := KindVariable
			if isFunc {
				kind = KindFunction
			} else if varKind == "const" {
				kind = KindConstant
			}

			endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, isFunc, isArrow)
			var calls []string
			if isFunc {
				calls = extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
			}
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         kind,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      endLine,
				Calls:        calls,
			})
			continue
		}

		// 7. Inside Class: Methods & Arrow properties
		if len(classStack) > 0 {
			curClass := classStack[len(classStack)-1].name

			// Arrow property: e.g. foo = () => {}
			if m := reTSJSPropArrow.FindStringSubmatch(trimmed); len(m) > 1 {
				methodName := m[1]
				if !tsExcludedMethodNames[methodName] {
					endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, true)
					calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
					symbols = append(symbols, Symbol{
						Name:         curClass + "." + methodName,
						Kind:         KindMethod,
						RelativePath: relPath,
						StartLine:    lineNum,
						EndLine:      endLine,
						Calls:        calls,
					})
					continue
				}
			}

			// Method: e.g. foo() {}
			if m := reTSJSMethod.FindStringSubmatch(trimmed); len(m) > 1 {
				methodName := m[1]
				if !tsExcludedMethodNames[methodName] {
					endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, false)
					calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
					symbols = append(symbols, Symbol{
						Name:         curClass + "." + methodName,
						Kind:         KindMethod,
						RelativePath: relPath,
						StartLine:    lineNum,
						EndLine:      endLine,
						Calls:        calls,
					})
					continue
				}
			}
		}
	}

	return symbols, imports
}

// computePythonEndLine tracks Python block indentation: a block ends at the last non-empty,
// non-comment line before a line with indentation <= declaration indentation.
func computePythonEndLine(lines []string, startIdx int, declIndent int) int {
	lastNonEmpty := startIdx + 1 // 1-indexed

	for j := startIdx + 1; j < len(lines); j++ {
		line := lines[j]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := 0
		for _, ch := range line {
			if ch == ' ' {
				lineIndent++
			} else if ch == '\t' {
				lineIndent += 4
			} else {
				break
			}
		}

		if lineIndent <= declIndent {
			break
		}
		lastNonEmpty = j + 1
	}

	return lastNonEmpty
}

// parsePythonSymbols extracts structural symbols, calls, and imports from Python source files.
func parsePythonSymbols(relPath, content string) ([]Symbol, []string) {
	masked := maskStringsAndComments(content, "py")
	lineOffsets := computeLineOffsets(content)
	lines := strings.Split(content, "\n")
	var symbols []Symbol
	var imports []string
	seenImports := make(map[string]bool)

	// Extract Python imports
	for _, line := range lines {
		if len(imports) >= 32 {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if m := rePythonImport.FindStringSubmatch(trimmed); len(m) > 1 {
			target := m[1]
			if target == "" && len(m) > 2 {
				target = m[2]
			}
			for _, part := range strings.Split(target, ",") {
				imp := strings.TrimSpace(part)
				if imp != "" && !seenImports[imp] {
					seenImports[imp] = true
					imports = append(imports, imp)
					if len(imports) >= 32 {
						break
					}
				}
			}
		}
	}

	type pyClassScope struct {
		name   string
		indent int
	}
	var classStack []pyClassScope

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineNum := i + 1
		lineIndent := 0
		for _, ch := range line {
			if ch == ' ' {
				lineIndent++
			} else if ch == '\t' {
				lineIndent += 4
			} else {
				break
			}
		}

		// Pop classes that have ended due to indentation
		for len(classStack) > 0 && lineIndent <= classStack[len(classStack)-1].indent {
			classStack = classStack[:len(classStack)-1]
		}

		// 1. Class
		if m := rePythonClass.FindStringSubmatch(trimmed); len(m) > 1 {
			name := m[1]
			endLine := computePythonEndLine(lines, i, lineIndent)
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         KindStruct,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      endLine,
			})
			classStack = append(classStack, pyClassScope{name: name, indent: lineIndent})
			continue
		}

		// 2. Def (function or method)
		if m := rePythonDef.FindStringSubmatch(trimmed); len(m) > 1 {
			name := m[1]
			endLine := computePythonEndLine(lines, i, lineIndent)
			calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)

			if len(classStack) > 0 {
				fullName := classStack[len(classStack)-1].name + "." + name
				symbols = append(symbols, Symbol{
					Name:         fullName,
					Kind:         KindMethod,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
					Calls:        calls,
				})
			} else if lineIndent == 0 {
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindFunction,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
					Calls:        calls,
				})
			}
			continue
		}
	}

	return symbols, imports
}

// parseCStyleSymbols extracts structural symbols, calls, and imports from Rust, PHP, Java, and C# source files.
func parseCStyleSymbols(relPath, content, ext string) ([]Symbol, []string) {
	lang := "c"
	switch ext {
	case ".rs":
		lang = "rs"
	case ".php":
		lang = "php"
	}

	masked := maskStringsAndComments(content, lang)
	lineOffsets := computeLineOffsets(content)
	lines := strings.Split(content, "\n")
	maskedLines := strings.Split(masked, "\n")

	var symbols []Symbol
	var imports []string
	seenImports := make(map[string]bool)

	// Extract language-specific imports
	for _, line := range lines {
		if len(imports) >= 32 {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		var match []string
		switch ext {
		case ".rs":
			match = reRustUse.FindStringSubmatch(trimmed)
		case ".php":
			match = rePHPUse.FindStringSubmatch(trimmed)
		case ".java":
			match = reJavaImport.FindStringSubmatch(trimmed)
		case ".cs":
			match = reCSUsing.FindStringSubmatch(trimmed)
		}

		if len(match) > 1 {
			target := strings.TrimSpace(match[1])
			if target != "" && !seenImports[target] {
				seenImports[target] = true
				imports = append(imports, target)
				if len(imports) >= 32 {
					break
				}
			}
		}
	}

	var classStack []classScope

	for i := 0; i < len(lines); i++ {
		lineNum := i + 1
		maskedLine := maskedLines[i]
		trimmed := strings.TrimSpace(maskedLine)
		if trimmed == "" {
			continue
		}

		for len(classStack) > 0 && lineNum > classStack[len(classStack)-1].endLine {
			classStack = classStack[:len(classStack)-1]
		}

		colOffset := strings.Index(maskedLine, trimmed)
		declOffset := lineOffsets[i] + colOffset

		switch ext {
		case ".rs":
			if m := reRustStruct.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindStruct,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := reRustEnum.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindType,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := reRustTrait.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindInterface,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := reRustImpl.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				classStack = append(classStack, classScope{name: name, endLine: endLine})
				continue
			}
			if m := reRustFn.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, false)
				calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
				if len(classStack) > 0 {
					symbols = append(symbols, Symbol{
						Name:         classStack[len(classStack)-1].name + "." + name,
						Kind:         KindMethod,
						RelativePath: relPath,
						StartLine:    lineNum,
						EndLine:      endLine,
						Calls:        calls,
					})
				} else {
					symbols = append(symbols, Symbol{
						Name:         name,
						Kind:         KindFunction,
						RelativePath: relPath,
						StartLine:    lineNum,
						EndLine:      endLine,
						Calls:        calls,
					})
				}
				continue
			}

		case ".php":
			if m := rePHPClass.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindStruct,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				classStack = append(classStack, classScope{name: name, endLine: endLine})
				continue
			}
			if m := rePHPInterface.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindInterface,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := rePHPTrait.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindInterface,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if len(classStack) > 0 {
				if m := rePHPMethod.FindStringSubmatch(trimmed); len(m) > 1 {
					name := m[1]
					endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, false)
					calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
					symbols = append(symbols, Symbol{
						Name:         classStack[len(classStack)-1].name + "." + name,
						Kind:         KindMethod,
						RelativePath: relPath,
						StartLine:    lineNum,
						EndLine:      endLine,
						Calls:        calls,
					})
					continue
				}
			}
			if m := rePHPFunc.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, false)
				calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindFunction,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
					Calls:        calls,
				})
				continue
			}

		case ".java":
			if m := reJavaClass.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindStruct,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				classStack = append(classStack, classScope{name: name, endLine: endLine})
				continue
			}
			if m := reJavaInterface.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindInterface,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := reJavaEnum.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindType,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := reJavaRecord.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindStruct,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if len(classStack) > 0 {
				if m := reJavaMethod.FindStringSubmatch(trimmed); len(m) > 1 {
					name := m[1]
					if !cstyleExcludedMethodNames[name] {
						endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, false)
						calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
						symbols = append(symbols, Symbol{
							Name:         classStack[len(classStack)-1].name + "." + name,
							Kind:         KindMethod,
							RelativePath: relPath,
							StartLine:    lineNum,
							EndLine:      endLine,
							Calls:        calls,
						})
						continue
					}
				}
			}

		case ".cs":
			if m := reCSClass.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindStruct,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				classStack = append(classStack, classScope{name: name, endLine: endLine})
				continue
			}
			if m := reCSStruct.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindStruct,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := reCSInterface.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindInterface,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if m := reCSEnum.FindStringSubmatch(trimmed); len(m) > 1 {
				name := m[1]
				endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, false, false)
				symbols = append(symbols, Symbol{
					Name:         name,
					Kind:         KindType,
					RelativePath: relPath,
					StartLine:    lineNum,
					EndLine:      endLine,
				})
				continue
			}
			if len(classStack) > 0 {
				if m := reCSMethod.FindStringSubmatch(trimmed); len(m) > 1 {
					name := m[1]
					if !cstyleExcludedMethodNames[name] {
						endLine := findBodyEndLine(masked, lineOffsets, lineNum, declOffset, true, false)
						calls := extractCallsFromSpan(masked, lineOffsets, lineNum, endLine)
						symbols = append(symbols, Symbol{
							Name:         classStack[len(classStack)-1].name + "." + name,
							Kind:         KindMethod,
							RelativePath: relPath,
							StartLine:    lineNum,
							EndLine:      endLine,
							Calls:        calls,
						})
						continue
					}
				}
			}
		}
	}

	return symbols, imports
}
