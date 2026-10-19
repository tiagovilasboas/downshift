// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package web embeds the dashboard's static files, so the server never has to
// guess a directory next to the binary or in the working directory.
package web

import "embed"

// FS holds the dashboard at its root (index.html, app.js, ...). Test files in
// this directory are not embedded.
//
//go:embed index.html app.js boot.js sw.js manifest.json favicon.svg brand
var FS embed.FS
