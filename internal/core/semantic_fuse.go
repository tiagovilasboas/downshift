// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "github.com/tiagovilasboas/downshift/internal/fuse"

// ClassifyWithSemantic runs the regex classifier plus the optional in-process
// hash semantic boost (DOWNSHIFT_MINILM). The boost policy lives in internal/fuse.
func ClassifyWithSemantic(prompt string) Classification { return fuse.Apply(prompt) }
