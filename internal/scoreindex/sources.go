// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package scoreindex

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Source names, in priority order. Scales differ between sources (Epoch ECI is
// about 118-167, the Artificial Analysis index about 4-58), so one comparison
// never mixes sources.
const (
	SourceEpoch      = "epoch-ai"
	SourceOpenRouter = "openrouter-aa"
)

// EpochURL, OpenRouterURL are the public endpoints. No key, no scraping.
const (
	EpochURL      = "https://epoch.ai/data/eci_scores.csv"
	OpenRouterURL = "https://openrouter.ai/api/v1/models"

	epochMaxBody      = 4 << 20
	openRouterMaxBody = 8 << 20
	maxRedirects      = 3
)

// Attribution strings shown by `downshift models scores --status`.
const (
	EpochAttribution      = "Epoch AI, 'Capabilities & benchmarking', https://epoch.ai/benchmarks (CC BY 4.0)"
	OpenRouterAttribution = "Model metadata: OpenRouter (https://openrouter.ai); intelligence index: Artificial Analysis (https://artificialanalysis.ai/). Local cache only."
)

// ErrNotModified is returned by a source whose server answered 304 to the
// cached validator. The caller keeps the cached rows of that source.
var ErrNotModified = errors.New("not modified")

// Row is one comparable score of one session model id.
type Row struct {
	ID      string    `json:"id"`
	Harness string    `json:"harness"`
	Score   float64   `json:"score"`
	Source  string    `json:"source"`
	Fetched time.Time `json:"fetched"`
	License string    `json:"license"`
	URL     string    `json:"url"`
}

// Source fetches the rows of one public source. It returns ErrNotModified when
// the cached copy is still current, and an error for an empty or unusable
// response (an empty answer must never replace a cache).
type Source interface {
	Name() string
	Fetch(ctx context.Context, c *http.Client) ([]Row, error)
}

// ETagger is implemented by sources that revalidate with an ETag. ETag returns
// the validator to store after Fetch.
type ETagger interface{ ETag() string }

// Meta returns license, url and attribution for a known source name.
func Meta(name string) (license, url, attribution string) {
	switch name {
	case SourceEpoch:
		return "CC-BY-4.0", EpochURL, EpochAttribution
	case SourceOpenRouter:
		return "OpenRouter public API; Artificial Analysis data, attribution required", OpenRouterURL, OpenRouterAttribution
	}
	return "", "", ""
}

// DefaultSources lists the sources in priority order. etag is the validator
// cached for Epoch ("" for none). version goes into the User-Agent.
func DefaultSources(m *IDMap, etag, version string) []Source {
	if m == nil {
		m = DefaultIDMap()
	}
	ua := userAgent(version)
	return []Source{
		&Epoch{URL: EpochURL, Map: m, UserAgent: ua, IfNoneMatch: etag},
		&OpenRouter{URL: OpenRouterURL, Map: m, UserAgent: ua},
	}
}

func userAgent(version string) string {
	if version == "" {
		version = "dev"
	}
	return "downshift/" + version + " (+https://github.com/tiagovilasboas/downshift)"
}

// get issues a GET with a body cap and a same-host redirect policy. It returns
// the response and its capped body.
func get(ctx context.Context, c *http.Client, url, ua, accept, inm string, max int64) (*http.Response, []byte, error) {
	if c == nil {
		c = &http.Client{}
	}
	cc := *c
	cc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errors.New("too many redirects")
		}
		first := via[0].URL
		if req.URL.Host != first.Host || req.URL.Scheme != first.Scheme {
			return fmt.Errorf("redirect to another host (%s) refused", req.URL.Host)
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", ua)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if inm != "" {
		req.Header.Set("If-None-Match", inm)
	}
	resp, err := cc.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return resp, nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return resp, nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return resp, nil, err
	}
	if int64(len(body)) > max {
		return resp, nil, fmt.Errorf("response larger than %d bytes", max)
	}
	return resp, body, nil
}

// Epoch reads the Epoch AI Capabilities Index CSV (CC BY 4.0).
type Epoch struct {
	URL         string
	Map         *IDMap
	UserAgent   string
	IfNoneMatch string
	Now         func() time.Time

	etag string
}

func (e *Epoch) Name() string { return SourceEpoch }

// ETag is the validator to cache after Fetch.
func (e *Epoch) ETag() string {
	if e.etag != "" {
		return e.etag
	}
	return e.IfNoneMatch
}

func (e *Epoch) Fetch(ctx context.Context, c *http.Client) ([]Row, error) {
	resp, body, err := get(ctx, c, e.URL, e.UserAgent, "text/csv", e.IfNoneMatch, epochMaxBody)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotModified {
		if v := resp.Header.Get("ETag"); v != "" {
			e.etag = v
		}
		return nil, ErrNotModified
	}
	rows, err := parseEpoch(body, e.Map, nowOr(e.Now))
	if err != nil {
		return nil, err
	}
	e.etag = resp.Header.Get("ETag")
	return rows, nil
}

func nowOr(f func() time.Time) time.Time {
	if f != nil {
		return f()
	}
	return time.Now()
}

// parseEpoch maps ECI rows to session ids. Unmapped models are dropped.
func parseEpoch(body []byte, m *IDMap, now time.Time) ([]Row, error) {
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("epoch csv header: %w", err)
	}
	col := map[string]int{}
	for i, h := range head {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	iModel, iDisplay, iECI := -1, -1, -1
	if i, ok := col["model"]; ok {
		iModel = i
	}
	if i, ok := col["display name"]; ok {
		iDisplay = i
	}
	if i, ok := col["eci"]; ok {
		iECI = i
	}
	if iECI < 0 || (iModel < 0 && iDisplay < 0) {
		return nil, errors.New("epoch csv: columns Model and eci not found")
	}
	license, url, _ := Meta(SourceEpoch)
	seen := map[string]bool{}
	var out []Row
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("epoch csv: %w", err)
		}
		if iECI >= len(rec) {
			continue
		}
		score, err := strconv.ParseFloat(strings.TrimSpace(rec[iECI]), 64)
		if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
			continue
		}
		var targets []Target
		for _, i := range []int{iModel, iDisplay} {
			if len(targets) == 0 && i >= 0 && i < len(rec) {
				targets = m.LookupEpoch(rec[i])
			}
		}
		for _, t := range targets {
			k := t.Harness + "\x00" + t.ID
			if seen[k] {
				continue // first (highest ECI) occurrence wins
			}
			seen[k] = true
			out = append(out, Row{ID: t.ID, Harness: t.Harness, Score: score, Source: SourceEpoch, Fetched: now, License: license, URL: url})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("epoch csv: no mapped rows")
	}
	return out, nil
}

// OpenRouter reads the public models API and takes the Artificial Analysis
// intelligence index it carries. The numbers stay in the local cache.
type OpenRouter struct {
	URL       string
	Map       *IDMap
	UserAgent string
	Now       func() time.Time
}

func (o *OpenRouter) Name() string { return SourceOpenRouter }

func (o *OpenRouter) Fetch(ctx context.Context, c *http.Client) ([]Row, error) {
	_, body, err := get(ctx, c, o.URL, o.UserAgent, "application/json", "", openRouterMaxBody)
	if err != nil {
		return nil, err
	}
	return parseOpenRouter(body, o.Map, nowOr(o.Now))
}

type orModels struct {
	Data []struct {
		ID         string `json:"id"`
		Benchmarks struct {
			AA *struct {
				Intelligence *float64 `json:"intelligence_index"`
			} `json:"artificial_analysis"`
		} `json:"benchmarks"`
	} `json:"data"`
}

func parseOpenRouter(body []byte, m *IDMap, now time.Time) ([]Row, error) {
	var doc orModels
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("openrouter json: %w", err)
	}
	license, url, _ := Meta(SourceOpenRouter)
	seen := map[string]bool{}
	var out []Row
	for _, d := range doc.Data {
		if d.Benchmarks.AA == nil || d.Benchmarks.AA.Intelligence == nil {
			continue
		}
		score := *d.Benchmarks.AA.Intelligence
		if math.IsNaN(score) || math.IsInf(score, 0) {
			continue
		}
		for _, t := range m.LookupOpenRouter(d.ID) {
			k := t.Harness + "\x00" + t.ID
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, Row{ID: t.ID, Harness: t.Harness, Score: score, Source: SourceOpenRouter, Fetched: now, License: license, URL: url})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("openrouter: no mapped rows with an intelligence index")
	}
	return out, nil
}
