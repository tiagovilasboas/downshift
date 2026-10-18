// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// Fallback patterns for non-Go code symbols
	rePyDef   = regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z0-9_]+)`)
	rePyClass = regexp.MustCompile(`^\s*class\s+([A-Za-z0-9_]+)`)
	reJSFunc  = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z0-9_]+)`)
	reJSClass = regexp.MustCompile(`^\s*(?:export\s+)?class\s+([A-Za-z0-9_]+)`)
	reJSConst = regexp.MustCompile(`^\s*(?:export\s+)?const\s+([A-Za-z0-9_]+)\s*=`)
	reType    = regexp.MustCompile(`^\s*(?:export\s+)?(?:type|interface)\s+([A-Za-z0-9_]+)`)
)

// ParseFile parses a source file from disk into an indexed Document.
// Raw file contents are used for term frequency and symbol extraction, but never stored.
func ParseFile(repoRoot, relPath string) (*Document, error) {
	fullPath := filepath.Join(repoRoot, relPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	hasher := sha256.New()
	hasher.Write(contentBytes)
	hashStr := hex.EncodeToString(hasher.Sum(nil))

	contentStr := string(contentBytes)
	termFreqs, totalTerms := TermFrequencies(contentStr)

	// Include path tokens in frequencies with bonus
	pathTokens := Tokenize(relPath)
	for _, pt := range pathTokens {
		termFreqs[pt] += 2
		totalTerms += 2
	}

	var symbols []Symbol
	ext := strings.ToLower(filepath.Ext(relPath))
	if ext == ".go" {
		symbols = parseGoAST(relPath, contentBytes)
	} else {
		symbols = parseFallbackSymbols(relPath, contentStr)
	}

	// Boost symbol name tokens
	for _, sym := range symbols {
		symTokens := Tokenize(sym.Name)
		for _, st := range symTokens {
			termFreqs[st] += 3
			totalTerms += 3
		}
	}

	doc := &Document{
		RelativePath: filepath.ToSlash(relPath),
		SHA256:       hashStr,
		Size:         info.Size(),
		ModTime:      info.ModTime().UTC().Truncate(time.Second),
		TermFreqs:    termFreqs,
		Symbols:      symbols,
		TotalTerms:   totalTerms,
	}

	return doc, nil
}

func parseGoAST(relPath string, src []byte) []Symbol {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		// If AST parse fails (syntax error), fallback to lexical scan
		return parseFallbackSymbols(relPath, string(src))
	}

	var symbols []Symbol

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := KindFunction
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = KindMethod
				recvType := exprToString(d.Recv.List[0].Type)
				if recvType != "" {
					name = recvType + "." + name
				}
			}
			startLine := fset.Position(d.Pos()).Line
			endLine := fset.Position(d.End()).Line
			symbols = append(symbols, Symbol{
				Name:         name,
				Kind:         kind,
				RelativePath: relPath,
				StartLine:    startLine,
				EndLine:      endLine,
			})

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					kind := KindType
					switch s.Type.(type) {
					case *ast.StructType:
						kind = KindStruct
					case *ast.InterfaceType:
						kind = KindInterface
					}
					startLine := fset.Position(s.Pos()).Line
					endLine := fset.Position(s.End()).Line
					symbols = append(symbols, Symbol{
						Name:         s.Name.Name,
						Kind:         kind,
						RelativePath: relPath,
						StartLine:    startLine,
						EndLine:      endLine,
					})
				case *ast.ValueSpec:
					kind := KindVariable
					if d.Tok == token.CONST {
						kind = KindConstant
					}
					for _, nameIdent := range s.Names {
						startLine := fset.Position(nameIdent.Pos()).Line
						endLine := fset.Position(nameIdent.End()).Line
						symbols = append(symbols, Symbol{
							Name:         nameIdent.Name,
							Kind:         kind,
							RelativePath: relPath,
							StartLine:    startLine,
							EndLine:      endLine,
						})
					}
				}
			}
		}
	}

	return symbols
}

func parseFallbackSymbols(relPath, content string) []Symbol {
	var symbols []Symbol
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 1

	for scanner.Scan() {
		line := scanner.Text()

		if m := rePyDef.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, Symbol{
				Name:         m[1],
				Kind:         KindFunction,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      lineNum,
			})
		} else if m := rePyClass.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, Symbol{
				Name:         m[1],
				Kind:         KindStruct,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      lineNum,
			})
		} else if m := reJSFunc.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, Symbol{
				Name:         m[1],
				Kind:         KindFunction,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      lineNum,
			})
		} else if m := reJSClass.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, Symbol{
				Name:         m[1],
				Kind:         KindStruct,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      lineNum,
			})
		} else if m := reType.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, Symbol{
				Name:         m[1],
				Kind:         KindType,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      lineNum,
			})
		} else if m := reJSConst.FindStringSubmatch(line); len(m) > 1 {
			symbols = append(symbols, Symbol{
				Name:         m[1],
				Kind:         KindConstant,
				RelativePath: relPath,
				StartLine:    lineNum,
				EndLine:      lineNum,
			})
		}

		lineNum++
	}

	return symbols
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return exprToString(e.X)
	default:
		return ""
	}
}
