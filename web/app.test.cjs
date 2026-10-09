// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');

test('all and Cursor totals exclude held and allow verdicts from applied shifts', () => {
  const elements = new Map();
  const document = {
    getElementById(id) {
      if (!elements.has(id)) elements.set(id, { innerHTML: '', style: {} });
      return elements.get(id);
    },
  };
  const context = vm.createContext({
    document,
    fetch: () => new Promise(() => {}),
    AbortSignal,
    EventSource: class { addEventListener() {} },
    setInterval() {},
    setTimeout() {},
  });
  vm.runInContext(readFileSync(join(__dirname, 'app.js'), 'utf8'), context);
  const timestamp = new Date().toISOString();
  const decision = (harness, verdict, applied, savings) => ({
    timestamp, harness, verdict, applied, estimated_savings: savings,
  });
  context.fixture = {
    harnesses: ['cursor', 'codex'], agents: null,
    switches: [
      decision('cursor', 'DOWNSHIFT', true, 0.8),
      decision('cursor', 'DOWNSHIFT', false, 0.4),
      decision('cursor', 'DOWNSHIFT', false, 0.7),
      decision('cursor', 'UPSHIFT', false, 0),
      decision('codex', 'DOWNSHIFT', true, 0.5),
    ],
    stats: { total: 5, down: 2, up: 0, est_usd: 0.013 },
  };
  const text = id => document.getElementById(id).innerHTML.replace(/<[^>]*>/g, '');
  vm.runInContext('renderMonitor(fixture)', context);
  assert.match(text('monitor-stats'), /5 events.*2↓.*0↑/);
  assert.match(text('economy-bar'), /2 downshifts/);

  vm.runInContext("setActiveHarness('cursor')", context);
  assert.match(text('monitor-stats'), /4 events.*1↓.*0↑.*cursor only/);
  assert.match(text('economy-bar'), /1 downshift/);
  assert.match(text('economy-bar'), /\$0\.01/);

  vm.runInContext("setActiveHarness('kirocrew')", context);
  assert.match(text('monitor-stats'), /0 events.*0↓.*0↑/);
  assert.equal(document.getElementById('economy-section').style.display, 'none');
  assert.equal(text('economy-bar'), '');
});
