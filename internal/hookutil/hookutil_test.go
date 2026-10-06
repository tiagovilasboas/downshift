// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookutil_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/hookutil"
)

func TestStringField(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{
			name: "present string value",
			m:    map[string]any{"model": "claude-haiku-4-5"},
			key:  "model",
			want: "claude-haiku-4-5",
		},
		{
			name: "absent key returns empty",
			m:    map[string]any{"model": "claude-haiku-4-5"},
			key:  "prompt",
			want: "",
		},
		{
			name: "nil map returns empty",
			m:    nil,
			key:  "model",
			want: "",
		},
		{
			name: "empty map returns empty",
			m:    map[string]any{},
			key:  "model",
			want: "",
		},
		{
			name: "value is int not string",
			m:    map[string]any{"count": 42},
			key:  "count",
			want: "",
		},
		{
			name: "value is bool not string",
			m:    map[string]any{"flag": true},
			key:  "flag",
			want: "",
		},
		{
			name: "value is nil",
			m:    map[string]any{"model": nil},
			key:  "model",
			want: "",
		},
		{
			name: "value is nested map not string",
			m:    map[string]any{"nested": map[string]any{"a": "b"}},
			key:  "nested",
			want: "",
		},
		{
			name: "empty string value",
			m:    map[string]any{"model": ""},
			key:  "model",
			want: "",
		},
		{
			name: "key with spaces",
			m:    map[string]any{"tool name": "Task"},
			key:  "tool name",
			want: "Task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hookutil.StringField(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("StringField(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

func TestTaskText(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		keys []string
		want string
	}{
		{
			name: "first key match",
			m:    map[string]any{"prompt": "task description", "description": "fallback"},
			keys: []string{"prompt", "description"},
			want: "task description",
		},
		{
			name: "second key match when first absent",
			m:    map[string]any{"description": "fallback text"},
			keys: []string{"prompt", "description"},
			want: "fallback text",
		},
		{
			name: "no keys match returns empty",
			m:    map[string]any{"other": "value"},
			keys: []string{"prompt", "description"},
			want: "",
		},
		{
			name: "empty string trimmed returns empty",
			m:    map[string]any{"prompt": "   "},
			keys: []string{"prompt"},
			want: "",
		},
		{
			name: "whitespace trimmed",
			m:    map[string]any{"prompt": "  task text  "},
			keys: []string{"prompt"},
			want: "task text",
		},
		{
			name: "first non-empty wins even if others exist",
			m:    map[string]any{"prompt": "main", "description": "ignored", "title": "also ignored"},
			keys: []string{"prompt", "description", "title"},
			want: "main",
		},
		{
			name: "skip empty values and use next",
			m:    map[string]any{"prompt": "", "description": "fallback"},
			keys: []string{"prompt", "description"},
			want: "fallback",
		},
		{
			name: "no keys provided returns empty",
			m:    map[string]any{"prompt": "text"},
			keys: []string{},
			want: "",
		},
		{
			name: "nil map returns empty",
			m:    nil,
			keys: []string{"prompt"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hookutil.TaskText(tt.m, tt.keys...)
			if got != tt.want {
				t.Errorf("TaskText(%v, %v) = %q, want %q", tt.m, tt.keys, got, tt.want)
			}
		})
	}
}
