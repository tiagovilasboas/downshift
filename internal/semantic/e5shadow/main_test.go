// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package e5shadow_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookctx"
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	restore := hookctx.Set(ctx)
	code := m.Run()
	restore()
	cancel()
	os.Exit(code)
}
