// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoFile(t *testing.T) {
	tempDir := t.TempDir()
	goSrc := `package testpkg

type ServerConfig struct {
	Port int
}

type Runner interface {
	Run() error
}

const DefaultPort = 8080

func NewServer(cfg ServerConfig) *ServerConfig {
	return &cfg
}

func (s *ServerConfig) Start() error {
	return nil
}
`
	filePath := filepath.Join(tempDir, "server.go")
	if err := os.WriteFile(filePath, []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(tempDir, "server.go")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if doc.RelativePath != "server.go" {
		t.Errorf("doc.RelativePath = %q, want 'server.go'", doc.RelativePath)
	}
	if doc.SHA256 == "" {
		t.Error("doc.SHA256 is empty")
	}

	symbolNames := make(map[string]SymbolKind)
	for _, sym := range doc.Symbols {
		symbolNames[sym.Name] = sym.Kind
	}

	expectedSymbols := map[string]SymbolKind{
		"ServerConfig":       KindStruct,
		"Runner":             KindInterface,
		"DefaultPort":        KindConstant,
		"NewServer":          KindFunction,
		"ServerConfig.Start": KindMethod,
	}

	for name, kind := range expectedSymbols {
		gotKind, ok := symbolNames[name]
		if !ok {
			t.Errorf("Expected symbol %q not found", name)
		} else if gotKind != kind {
			t.Errorf("Symbol %q kind = %v, want %v", name, gotKind, kind)
		}
	}
}

func TestParseFallbackFile(t *testing.T) {
	tempDir := t.TempDir()
	pySrc := `
class PaymentProcessor:
    def process(self):
        pass

def calculate_fee():
    return 10
`
	filePath := filepath.Join(tempDir, "payment.py")
	if err := os.WriteFile(filePath, []byte(pySrc), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(tempDir, "payment.py")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(doc.Symbols) < 3 {
		t.Errorf("Expected at least 3 symbols, got %d", len(doc.Symbols))
	}
}

func TestParseGoFile_LineAccuracy(t *testing.T) {
	tempDir := t.TempDir()
	goSrc := `package testpkg

type Status int

const (
	StatusPending Status = iota
	StatusActive
)

var GlobalManager string

type Config struct {
	Timeout int
}

type Processor interface {
	Process() error
}

func ExecuteTask(task string) error {
	return nil
}

func (c *Config) Validate() bool {
	return c.Timeout > 0
}
`
	filePath := filepath.Join(tempDir, "accuracy.go")
	if err := os.WriteFile(filePath, []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(tempDir, "accuracy.go")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	symMap := make(map[string]Symbol)
	for _, s := range doc.Symbols {
		symMap[s.Name] = s
	}

	// 1. type Status
	if sym, ok := symMap["Status"]; !ok {
		t.Error("missing symbol 'Status'")
	} else {
		if sym.Kind != KindType {
			t.Errorf("Status kind = %v, want KindType", sym.Kind)
		}
		if sym.StartLine != 3 {
			t.Errorf("Status StartLine = %d, want 3", sym.StartLine)
		}
	}

	// 2. struct Config
	if sym, ok := symMap["Config"]; !ok {
		t.Error("missing symbol 'Config'")
	} else {
		if sym.Kind != KindStruct {
			t.Errorf("Config kind = %v, want KindStruct", sym.Kind)
		}
		if sym.StartLine != 12 {
			t.Errorf("Config StartLine = %d, want 12", sym.StartLine)
		}
	}

	// 3. interface Processor
	if sym, ok := symMap["Processor"]; !ok {
		t.Error("missing symbol 'Processor'")
	} else {
		if sym.Kind != KindInterface {
			t.Errorf("Processor kind = %v, want KindInterface", sym.Kind)
		}
		if sym.StartLine != 16 {
			t.Errorf("Processor StartLine = %d, want 16", sym.StartLine)
		}
	}

	// 4. func ExecuteTask
	if sym, ok := symMap["ExecuteTask"]; !ok {
		t.Error("missing symbol 'ExecuteTask'")
	} else {
		if sym.Kind != KindFunction {
			t.Errorf("ExecuteTask kind = %v, want KindFunction", sym.Kind)
		}
		if sym.StartLine != 20 {
			t.Errorf("ExecuteTask StartLine = %d, want 20", sym.StartLine)
		}
	}

	// 5. method Config.Validate
	if sym, ok := symMap["Config.Validate"]; !ok {
		t.Error("missing symbol 'Config.Validate'")
	} else {
		if sym.Kind != KindMethod {
			t.Errorf("Config.Validate kind = %v, want KindMethod", sym.Kind)
		}
		if sym.StartLine != 24 {
			t.Errorf("Config.Validate StartLine = %d, want 24", sym.StartLine)
		}
	}
}

func TestParseFallback_JSTSPHP(t *testing.T) {
	tempDir := t.TempDir()

	// 1. TypeScript file
	tsSrc := `
export interface UserSession {
    id: string;
    token: string;
}

export type AuthHeader = string;

export class SessionService {
    verify() {}
}

export async function authenticateToken(token: string) {
    return true;
}
`
	tsPath := filepath.Join(tempDir, "session.ts")
	if err := os.WriteFile(tsPath, []byte(tsSrc), 0644); err != nil {
		t.Fatal(err)
	}

	tsDoc, err := ParseFile(tempDir, "session.ts")
	if err != nil {
		t.Fatalf("ParseFile ts failed: %v", err)
	}

	tsSyms := make(map[string]SymbolKind)
	for _, s := range tsDoc.Symbols {
		tsSyms[s.Name] = s.Kind
	}
	if tsSyms["UserSession"] != KindType {
		t.Errorf("UserSession = %v, want KindType", tsSyms["UserSession"])
	}
	if tsSyms["AuthHeader"] != KindType {
		t.Errorf("AuthHeader = %v, want KindType", tsSyms["AuthHeader"])
	}
	if tsSyms["SessionService"] != KindStruct {
		t.Errorf("SessionService = %v, want KindStruct", tsSyms["SessionService"])
	}
	if tsSyms["authenticateToken"] != KindFunction {
		t.Errorf("authenticateToken = %v, want KindFunction", tsSyms["authenticateToken"])
	}

	// 2. JavaScript file
	jsSrc := `
class OrderManager {
    constructor() {}
}

const MAX_ORDERS = 100;

function calculateDiscount(order) {
    return order.total * 0.1;
}
`
	jsPath := filepath.Join(tempDir, "orders.js")
	if err := os.WriteFile(jsPath, []byte(jsSrc), 0644); err != nil {
		t.Fatal(err)
	}

	jsDoc, err := ParseFile(tempDir, "orders.js")
	if err != nil {
		t.Fatalf("ParseFile js failed: %v", err)
	}

	jsSyms := make(map[string]SymbolKind)
	for _, s := range jsDoc.Symbols {
		jsSyms[s.Name] = s.Kind
	}
	if jsSyms["OrderManager"] != KindStruct {
		t.Errorf("OrderManager = %v, want KindStruct", jsSyms["OrderManager"])
	}
	if jsSyms["MAX_ORDERS"] != KindConstant {
		t.Errorf("MAX_ORDERS = %v, want KindConstant", jsSyms["MAX_ORDERS"])
	}
	if jsSyms["calculateDiscount"] != KindFunction {
		t.Errorf("calculateDiscount = %v, want KindFunction", jsSyms["calculateDiscount"])
	}

	// 3. PHP file
	phpSrc := `<?php
class PaymentGateway {
    public function execute() {}
}

function processRefund($chargeId) {
    return true;
}
`
	phpPath := filepath.Join(tempDir, "gateway.php")
	if err := os.WriteFile(phpPath, []byte(phpSrc), 0644); err != nil {
		t.Fatal(err)
	}

	phpDoc, err := ParseFile(tempDir, "gateway.php")
	if err != nil {
		t.Fatalf("ParseFile php failed: %v", err)
	}

	phpSyms := make(map[string]SymbolKind)
	for _, s := range phpDoc.Symbols {
		phpSyms[s.Name] = s.Kind
	}
	if phpSyms["PaymentGateway"] != KindStruct {
		t.Errorf("PaymentGateway = %v, want KindStruct", phpSyms["PaymentGateway"])
	}
	if phpSyms["processRefund"] != KindFunction {
		t.Errorf("processRefund = %v, want KindFunction", phpSyms["processRefund"])
	}
}

