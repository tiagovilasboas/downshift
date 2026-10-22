// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dashboard/*
var dashboardAssets embed.FS

// spaHandler serves the embedded SPA assets with proper fallback to index.html.
func spaHandler() http.Handler {
	// Strip the "dashboard" prefix from embedded paths
	sub, err := fs.Sub(dashboardAssets, "dashboard")
	if err != nil {
		// If assets not embedded, return a placeholder
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Downshift Dashboard</title></head>
<body style="background:#0A0A0A;color:#fff;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center">
<h1>Dashboard not built</h1>
<p>Run <code style="background:#1A1A1A;padding:4px 8px;border-radius:4px">cd dashboard && pnpm build</code> first</p>
</div>
</body>
</html>`))
		})
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Serve static assets directly
		if strings.HasPrefix(path, "/assets/") ||
			path == "/favicon.svg" ||
			path == "/logo-full.svg" ||
			path == "/logo-mark.svg" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Try to serve the file, fallback to index.html for SPA routes
		if path != "/" {
			// Check if file exists
			if _, err := fs.Stat(sub, strings.TrimPrefix(path, "/")); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for all other routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
