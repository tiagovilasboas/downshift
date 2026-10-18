// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"reflect"
	"testing"
)

func TestTokenizeIdentifiers(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"HTTPServer", []string{"http", "server"}},
		{"userID", []string{"user", "id"}},
		{"processPIXPayment", []string{"process", "pix", "payment"}},
		{"getUserById", []string{"get", "user", "by", "id"}},
		{"payment_service", []string{"payment", "service"}},
		{"PaymentService", []string{"payment", "service"}},
		{"kebab-case-variable", []string{"kebab", "case", "variable"}},
		{"internal/codeintel/tokenize.go", []string{"internal", "codeintel", "tokenize", "go"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := Tokenize(tt.input)
		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("Tokenize(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestTokenizeStopwords(t *testing.T) {
	input := "the server is running with a timeout for the user"
	got := Tokenize(input)
	// "the", "is", "with", "a", "for", "the" should be filtered out
	expected := []string{"server", "running", "timeout", "user"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Tokenize(%q) = %v, want %v", input, got, expected)
	}
}

func TestTermFrequencies(t *testing.T) {
	input := "payment service processes payment transactions and payment logs"
	freqs, total := TermFrequencies(input)
	if total != 7 {
		t.Errorf("TermFrequencies total = %d, want 7", total)
	}
	if freqs["payment"] != 3 {
		t.Errorf("TermFrequencies['payment'] = %d, want 3", freqs["payment"])
	}
	if freqs["service"] != 1 {
		t.Errorf("TermFrequencies['service'] = %d, want 1", freqs["service"])
	}
}

func TestTokenizeDomainTerms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Portuguese PIX refund function",
			input:    "processarReembolsoPIX",
			expected: []string{"processar", "reembolso", "pix"},
		},
		{
			name:     "Portuguese boleto validation",
			input:    "validarBoletoBancario",
			expected: []string{"validar", "boleto", "bancario"},
		},
		{
			name:     "Portuguese split calculation with snake_case",
			input:    "taxa_split_antecipacao",
			expected: []string{"taxa", "split", "antecipacao"},
		},
		{
			name:     "Portuguese credit card recurrence",
			input:    "cartaoCreditoRecorrente",
			expected: []string{"cartao", "credito", "recorrente"},
		},
		{
			name:     "Portuguese bank reconciliation",
			input:    "reconciliacao_extrato_bancario",
			expected: []string{"reconciliacao", "extrato", "bancario"},
		},
		{
			name:     "English chargeback dispute",
			input:    "chargeback_dispute_resolution",
			expected: []string{"chargeback", "dispute", "resolution"},
		},
		{
			name:     "English auth token with acronym",
			input:    "authBearerToken",
			expected: []string{"auth", "bearer", "token"},
		},
		{
			name:     "English idempotency key store",
			input:    "idempotency_key_store",
			expected: []string{"idempotency", "key", "store"},
		},
		{
			name:     "English circuit breaker state",
			input:    "circuitBreakerState",
			expected: []string{"circuit", "breaker", "state"},
		},
		{
			name:     "SCREAMING_SNAKE_CASE constants",
			input:    "MAX_RETRIES_EXCEEDED",
			expected: []string{"max", "retries", "exceeded"},
		},
		{
			name:     "Identifiers containing digits and acronyms",
			input:    "sha256HashValidator",
			expected: []string{"sha256", "hash", "validator"},
		},
		{
			name:     "Deep nested path with version",
			input:    "internal/api/v2/payment/webhook_receiver.go",
			expected: []string{"internal", "api", "v2", "payment", "webhook", "receiver", "go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tokenize(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

