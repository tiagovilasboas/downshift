// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package ccsource reads Claude Code transcripts as replay cases. It is the
// only place that knows their shape: a parent session records each Agent call
// (its prompt, its subagent_type, an explicit model if any) and the subagent's
// own transcript records the usage and the model it ran on.
//
// A spawn becomes a case only when its baseline is certain: the model named in
// the call, or else the parent's model at the time of the call. A child whose
// model differs from that baseline was rewritten by a hook, or pinned by an
// agent definition; the transcript does not say which, so it is counted as
// skipped rather than guessed.
//
// Prompts stay in memory. Case ids are hashes of the agent id.
package ccsource

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/replay"
)

// Source reads every parent transcript under Root. Cmp decides whether two
// model names are the same model, so an alias ("sonnet") and its full id count
// as one.
type Source struct {
	Root string
	Cmp  replay.Comparer
}

// New returns a source over a Claude Code projects directory.
func New(root string, cmp replay.Comparer) Source { return Source{Root: root, Cmp: cmp} }

const syntheticModel = "<synthetic>"

type block struct {
	Type      string         `json:"type"`
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Input     map[string]any `json:"input"`
	ToolUseID string         `json:"tool_use_id"`
}

type usageJSON struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheRead  int64 `json:"cache_read_input_tokens"`
	CacheWrite int64 `json:"cache_creation_input_tokens"`
}

type message struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *usageJSON      `json:"usage"`
}

type entry struct {
	Type          string          `json:"type"`
	Message       *message        `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type toolResult struct {
	AgentID       string `json:"agentId"`
	ResolvedModel string `json:"resolvedModel"`
}

// pending is one Agent call seen in the parent, waiting for its result.
type pending struct {
	prompt        string
	subagentType  string
	explicitModel string
	parentModel   string
}

// Cases walks every parent transcript and returns the usable spawns.
func (s Source) Cases(ctx context.Context) (replay.Corpus, error) {
	out := replay.Corpus{Skipped: map[replay.SkipReason]int{}}
	err := filepath.WalkDir(s.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") || strings.Contains(path, string(filepath.Separator)+"subagents"+string(filepath.Separator)) {
			return nil
		}
		return s.readParent(path, &out)
	})
	return out, err
}

func (s Source) readParent(path string, out *replay.Corpus) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	pend := map[string]pending{}
	type link struct {
		id    string
		ptool string
	}
	var links []link
	parentModel := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e entry
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Message == nil {
			continue
		}
		var blocks []block
		_ = json.Unmarshal(e.Message.Content, &blocks)
		switch e.Type {
		case "assistant":
			if m := e.Message.Model; m != "" && m != syntheticModel {
				parentModel = m
			}
			for _, b := range blocks {
				if b.Type != "tool_use" || (b.Name != "Agent" && b.Name != "Task") {
					continue
				}
				pend[b.ID] = pending{
					prompt:        str(b.Input, "prompt"),
					subagentType:  str(b.Input, "subagent_type"),
					explicitModel: str(b.Input, "model"),
					parentModel:   parentModel,
				}
			}
		case "user":
			var tr toolResult
			_ = json.Unmarshal(e.ToolUseResult, &tr)
			if tr.AgentID == "" {
				continue
			}
			for _, b := range blocks {
				if b.Type == "tool_result" {
					if _, ok := pend[b.ToolUseID]; ok {
						links = append(links, link{id: tr.AgentID, ptool: b.ToolUseID})
					}
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	base := strings.TrimSuffix(path, ".jsonl")
	for _, l := range links {
		p := pend[l.ptool]
		c, reason, ok := s.caseFor(base, l.id, p)
		if !ok {
			out.Skipped[reason]++
			continue
		}
		out.Cases = append(out.Cases, c)
	}
	return nil
}

// caseFor builds one case from a linked subagent transcript, or the reason it
// cannot be used.
func (s Source) caseFor(parentBase, agentID string, p pending) (replay.Case, replay.SkipReason, bool) {
	if strings.TrimSpace(p.prompt) == "" {
		return replay.Case{}, replay.SkipNoPrompt, false
	}
	// A plugin agent type (name:type) may pin its own model; the transcript
	// does not carry the pin, so its baseline is not certain.
	if strings.Contains(p.subagentType, ":") {
		return replay.Case{}, replay.SkipPluginAgent, false
	}
	baseline := p.explicitModel
	if baseline == "" {
		baseline = p.parentModel
	}
	if baseline == "" || baseline == syntheticModel {
		return replay.Case{}, replay.SkipNoBaseline, false
	}
	usage, ran, err := readChild(parentBase + "/subagents/agent-" + agentID + ".jsonl")
	if err != nil || ran == "" {
		return replay.Case{}, replay.SkipNoRan, false
	}
	if !s.Cmp.Known(baseline) || !s.Cmp.Known(ran) {
		return replay.Case{}, replay.SkipUnpricedModel, false
	}
	if s.Cmp.Compare(baseline, ran) != replay.Same {
		return replay.Case{}, replay.SkipModelDiffers, false
	}
	if usage.Total() == 0 {
		return replay.Case{}, replay.SkipNoUsage, false
	}
	sum := sha256.Sum256([]byte(agentID))
	return replay.Case{
		ID:        hex.EncodeToString(sum[:8]),
		AgentType: p.subagentType,
		Prompt:    p.prompt,
		Baseline:  baseline,
		Ran:       ran,
		Tokens:    usage,
	}, "", true
}

// readChild sums the usage of one subagent transcript. Claude Code writes a
// message more than once while it streams, so each message id counts once, with
// the last copy's usage.
func readChild(path string) (replay.Tokens, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return replay.Tokens{}, "", err
	}
	defer f.Close()
	last := map[string]usageJSON{}
	ran := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e entry
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Type != "assistant" || e.Message == nil {
			continue
		}
		if m := e.Message.Model; m != "" && m != syntheticModel {
			ran = m
		}
		if e.Message.Usage != nil && e.Message.ID != "" {
			last[e.Message.ID] = *e.Message.Usage
		}
	}
	if err := sc.Err(); err != nil {
		return replay.Tokens{}, "", err
	}
	var t replay.Tokens
	for _, u := range last {
		t.Input += u.Input
		t.Output += u.Output
		t.CacheRead += u.CacheRead
		t.CacheWrite += u.CacheWrite
	}
	return t, ran, nil
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}
