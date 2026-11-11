// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Repo is the repository the releases come from; DefaultBase is its page.
const (
	Repo        = "tiagovilasboas/downshift-ai"
	DefaultBase = "https://github.com/" + Repo
)

// maxArchiveBytes caps any single download.
const maxArchiveBytes = 128 << 20

// Source reads releases from a GitHub repository page. No API calls: the
// /releases/latest redirect names the tag, as install.sh does, so anonymous
// rate limits do not apply.
type Source struct {
	Base   string
	Client *http.Client
}

// NewSource returns a Source for the published repository.
func NewSource() Source {
	return Source{Base: DefaultBase, Client: &http.Client{Timeout: 2 * time.Minute}}
}

// LatestTag follows /releases/latest one hop and reads the tag from the
// Location header (…/releases/tag/vX.Y.Z).
func (s Source) LatestTag() (string, error) {
	if err := s.checkBase(); err != nil {
		return "", err
	}
	c := *s.client()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Get(s.Base + "/releases/latest")
	if err != nil {
		return "", fmt.Errorf("asking GitHub for the latest release: %w", err)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	_, tag, ok := strings.Cut(loc, "/releases/tag/")
	if !ok || tag == "" || strings.ContainsAny(tag, "/?#") {
		return "", errors.New("GitHub did not name a latest release")
	}
	if t, err := url.PathUnescape(tag); err == nil {
		tag = t
	}
	return tag, nil
}

// Download fetches one release asset for tag.
func (s Source) Download(tag, asset string) ([]byte, error) {
	if err := s.checkBase(); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/releases/download/%s/%s", s.Base, url.PathEscape(tag), url.PathEscape(asset))
	c := *s.client()
	c.CheckRedirect = httpsOnly
	resp, err := c.Get(u)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", asset, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: HTTP %d", asset, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", asset, err)
	}
	if len(data) > maxArchiveBytes {
		return nil, fmt.Errorf("downloading %s: larger than %d MB", asset, maxArchiveBytes>>20)
	}
	return data, nil
}

func (s Source) checkBase() error {
	if !strings.HasPrefix(s.Base, "https://") {
		return fmt.Errorf("release source %q is not https", s.Base)
	}
	return nil
}

func (s Source) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

// httpsOnly refuses a redirect that leaves https (release downloads hop to a
// CDN host) and stops long chains.
func httpsOnly(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing a redirect to %s", req.URL.Scheme)
	}
	if len(via) >= 10 {
		return errors.New("too many redirects")
	}
	return nil
}
