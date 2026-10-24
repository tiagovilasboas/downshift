// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The engine refuses a `$` handed to another file as a value or read through a
// parameter (`host.process`), so the data client gets a port made of closures that
// spell every call as `$.noun.event(...)`.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { EngineInterface } from 'claude-code'
import type { HostPort } from '../../shared/data'

export function hostPort($: EngineInterface): HostPort {
  return {
    process: { run: (argv, init) => $.process.run(argv, init) },
    // The client reads one variable, DOWNSHIFT_BIN; the engine wants the name literal.
    env: { get: () => $.env.get('DOWNSHIFT_BIN') },
  }
}
