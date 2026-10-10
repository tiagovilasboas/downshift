#!/bin/sh
# Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
# harness-downshift by Tiago de Carvalho Vilas Boas
# https://github.com/tiagovilasboas/downshift
# Register this absolute file path in agent.kiro_hooks.preToolUse.
set -eu
router_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec "$router_dir/downshift" kirocrew
