// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxOutput bounds what a source reads from a CLI or file.
const maxOutput = 4 << 20

// CodexCache reads the model list Codex keeps for the signed-in account
// (~/.codex/models_cache.json). Only entries Codex itself shows in its picker
// (visibility "list") are reported.
type CodexCache struct{ Path string }

func (CodexCache) Harness() string { return "codex" }
func (CodexCache) Name() string    { return "codex models_cache.json" }

func (c CodexCache) Discover(context.Context) ([]Model, error) {
	path := c.Path
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".codex", "models_cache.json")
	}
	data, err := readBounded(path)
	if err != nil {
		return nil, err
	}
	return parseCodexCache(data)
}

func parseCodexCache(data []byte) ([]Model, error) {
	var f struct {
		Models []struct {
			Slug       string `json:"slug"`
			Display    string `json:"display_name"`
			Visibility string `json:"visibility"`
			Tier       string `json:"tier"`
			Efforts    []struct {
				Effort string `json:"effort"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("codex cache: %w", err)
	}
	var out []Model
	for _, m := range f.Models {
		if m.Slug == "" || m.Visibility != "list" {
			continue
		}
		model := Model{ID: m.Slug, Display: m.Display, Tier: m.Tier}
		for _, level := range m.Efforts {
			if level.Effort != "" {
				model.Efforts = append(model.Efforts, level.Effort)
			}
		}
		out = append(out, model)
	}
	if len(out) == 0 {
		return nil, errors.New("codex cache: no listed models")
	}
	return out, nil
}

// KiroCLI runs `kiro-cli chat --list-models --format json`, which reports the
// models the Kiro account can select. "auto" is Kiro's own router, not a
// model, and is skipped.
type KiroCLI struct{ Bin string }

func (KiroCLI) Harness() string { return "kirocrew" }
func (KiroCLI) Name() string    { return "kiro-cli chat --list-models" }

func (k KiroCLI) Discover(ctx context.Context) ([]Model, error) {
	bin := k.Bin
	if bin == "" {
		bin = defaultKiroCLI()
	}
	// The list command opens a sign-in page when the CLI has no session in
	// this context, so it runs only after whoami confirms one. Any other
	// binary, including the PATH name, is checked the same way.
	if !signedInKiroCLI(bin) {
		return nil, errKiroNotSignedIn
	}
	out, err := runCLI(ctx, bin, "chat", "--list-models", "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseKiroList(out)
}

func parseKiroList(data []byte) ([]Model, error) {
	var f struct {
		Models []struct {
			ID   string `json:"model_id"`
			Name string `json:"model_name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("kiro list: %w", err)
	}
	var out []Model
	for _, m := range f.Models {
		if m.ID == "" || m.ID == "auto" {
			continue
		}
		out = append(out, Model{ID: m.ID, Display: m.Name})
	}
	if len(out) == 0 {
		return nil, errors.New("kiro list: no models")
	}
	return out, nil
}

// GrokCLI runs `grok models`, which lists without authentication.
type GrokCLI struct{ Bin string }

func (GrokCLI) Harness() string { return "grok" }
func (GrokCLI) Name() string    { return "grok models" }

func (g GrokCLI) Discover(ctx context.Context) ([]Model, error) {
	bin := g.Bin
	if bin == "" {
		bin = "grok"
	}
	out, err := runCLI(ctx, bin, "models")
	if err != nil {
		return nil, err
	}
	return parseGrokList(out)
}

var grokLine = regexp.MustCompile(`^\s*[*-]\s+(\S+)`)

// parseGrokList reads the "Available models:" block: lines like
// "  * grok-4.6 (default)" and "  - grok-4.5".
func parseGrokList(data []byte) ([]Model, error) {
	var out []Model
	inBlock := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "Available models") {
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}
		if m := grokLine.FindStringSubmatch(line); m != nil {
			out = append(out, Model{ID: m[1]})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("grok list: no models")
	}
	return out, nil
}

// Defaults is the source registry `models discover` and `setup` use when no
// harness is named. Every harness with a verified listing mechanism belongs
// here, whatever its maturity; a harness without one (Antigravity) is absent
// and relies on the hook payload or the operator file. The OpenRouter
// public list is added only when DOWNSHIFT_DISCOVERY_OPENROUTER=1, so the
// default run makes no new network call. Failed sources preserve their cache;
// discovery does not imply included credit.
func Defaults() []Source {
	sources := []Source{AnthropicAPI{}, CodexCache{}, CursorCLI{}, GrokCLI{}, KiroCLI{}, OpenCodeCLI{}}
	if openRouterEnabled() {
		sources = append(sources, OpenRouterAPI{})
	}
	return sources
}
