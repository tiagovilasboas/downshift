// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
		out = append(out, Model{ID: m.Slug, Display: m.Display})
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
		bin = "kiro-cli"
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

// AnthropicAPI lists models through the Anthropic Models API. It needs an API
// key (ANTHROPIC_API_KEY); subscription-only Claude Code sessions have no
// listing endpoint and are skipped. The API reports created_at, which is how
// the newest model of a family is chosen without parsing names.
type AnthropicAPI struct {
	URL    string // default https://api.anthropic.com/v1/models
	Key    string // default $ANTHROPIC_API_KEY
	Client *http.Client
}

func (AnthropicAPI) Harness() string { return "claude-code" }
func (AnthropicAPI) Name() string    { return "Anthropic Models API" }

func (a AnthropicAPI) Discover(ctx context.Context) ([]Model, error) {
	key := a.Key
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key == "" {
		return nil, errors.New("no ANTHROPIC_API_KEY (subscription sessions cannot be listed)")
	}
	url := a.URL
	if url == "" {
		url = "https://api.anthropic.com/v1/models?limit=1000"
	}
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from Anthropic Models API", resp.StatusCode)
	}
	var env struct {
		Data []struct {
			ID      string    `json:"id"`
			Display string    `json:"display_name"`
			Created time.Time `json:"created_at"`
		} `json:"data"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxOutput))
	if err := dec.Decode(&env); err != nil {
		return nil, fmt.Errorf("anthropic list: %w", err)
	}
	var out []Model
	for _, m := range env.Data {
		if m.ID != "" {
			out = append(out, Model{ID: m.ID, Display: m.Display, CreatedAt: m.Created})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("anthropic list: no models")
	}
	return out, nil
}

// Defaults is the source set `models discover` uses when none is named.
// Harnesses without a verified listing mechanism (Cursor needs a login and its
// output format is unverified; Antigravity has none) are absent: they keep
// using session-models.json.
func Defaults() []Source {
	return []Source{AnthropicAPI{}, CodexCache{}, KiroCLI{}, GrokCLI{}}
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(io.LimitReader(f, maxOutput)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func runCLI(ctx context.Context, bin string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("%s not installed", bin)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdout, n: maxOutput}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.w.Len()+len(p) > l.n {
		return 0, errors.New("output too large")
	}
	return l.w.Write(p)
}
