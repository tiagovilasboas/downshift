// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
// Downshift by Tiago Vilas Boas
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
  const decision = (harness, verdict, applied) => ({
    timestamp, harness, verdict, applied,
    quota: applied ? 'available' : 'unknown',
    honor: 'unobserved',
    usage: 'unobserved',
  });
  context.fixture = {
    harnesses: ['cursor', 'codex'], agents: null,
    switches: [
      decision('cursor', 'DOWNSHIFT', true),
      decision('cursor', 'DOWNSHIFT', false),
      decision('cursor', 'DOWNSHIFT', false),
      decision('cursor', 'UPSHIFT', false),
      decision('codex', 'DOWNSHIFT', true),
    ],
    stats: { total: 5, down: 2, up: 0 },
  };
  const text = id => document.getElementById(id).innerHTML.replace(/<[^>]*>/g, '');
  vm.runInContext('renderMonitor(fixture)', context);
  assert.match(text('monitor-stats'), /5 events.*2↓.*0↑/);

  vm.runInContext("setActiveHarness('cursor')", context);
  assert.match(text('monitor-stats'), /4 events.*1↓.*0↑.*cursor only/);

  vm.runInContext("setActiveHarness('kirocrew')", context);
  assert.match(text('monitor-stats'), /0 events.*0↓.*0↑/);
});

test('tokens saved come from the server per tab, with no dollar figure', () => {
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
  const block = (line, types) => ({ tokens: { children: 2 }, line, by_agent_type: types });
  context.fixture = {
    harnesses: ['claude-code', 'cursor'], agents: [], switches: [],
    stats: {
      total: 0, down: 0, up: 0, dollars: 'downshift report',
      tokens_saved: block('tokens saved: work 62% · cache 58% (107k of 185k)', [
        { agent_type: 'general-purpose', line: 'general-purpose: work 70% · cache 64% (2 of 3 agents)' },
        { agent_type: 'Explore', line: 'Explore: work 0% · cache 0% (0 of 1 agent)' },
      ]),
      tokens_saved_by_harness: {
        'claude-code': block('tokens saved: work 62% · cache 58% (107k of 185k)', [
          { agent_type: 'untyped', line: 'untyped: work 62% · cache 58% (2 of 4 agents)' },
        ]),
      },
    },
  };
  const text = id => document.getElementById(id).innerHTML.replace(/<[^>]*>/g, '');
  vm.runInContext('renderMonitor(fixture)', context);
  assert.equal(document.getElementById('tokens-section').style.display, '');
  assert.match(text('tokens-saved'), /tokens saved: work 62% · cache 58% \(107k of 185k\)/);
  assert.match(text('tokens-saved'), /general-purpose: work 70% · cache 64% \(2 of 3 agents\)/);
  assert.match(text('tokens-saved'), /dollars: downshift report/);
  assert.doesNotMatch(text('tokens-saved'), /\$\d/);

  // One untyped row adds nothing to the total line, so it is not drawn.
  vm.runInContext("setActiveHarness('claude-code')", context);
  assert.match(text('tokens-saved'), /tokens saved: work 62%/);
  assert.doesNotMatch(text('tokens-saved'), /untyped/);

  // No priced child tokens for the harness: no figure, never a zero.
  vm.runInContext("setActiveHarness('cursor')", context);
  assert.equal(document.getElementById('tokens-section').style.display, 'none');
  assert.equal(text('tokens-saved'), '');
});

test('unobserved harness does not inflate honor or usage counts', () => {
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
  const row = (harness, quota, honor) => ({
    timestamp, harness, verdict: 'DOWNSHIFT', applied: true,
    quota, honor, usage: 'unobserved',
  });
  context.fixture = {
    harnesses: ['cursor', 'claude-code'], agents: [],
    switches: [
      row('cursor', 'held', 'unobserved'),
      row('cursor', 'unknown', 'unobserved'),
      { ...row('claude-code', 'available', 'observed'), usage: 'observed' },
    ],
    stats: { total: 3, down: 3, up: 0 },
  };
  const text = id => document.getElementById(id).innerHTML.replace(/<[^>]*>/g, '');
  vm.runInContext('renderMonitor(fixture)', context);
  assert.match(text('monitor-stats'), /honor observed 1 · quota available 1 held 1 unknown 1 · usage observed 1/);
  // No tokens_saved block from the server: the page draws no figure.
  assert.equal(document.getElementById('tokens-section').style.display, 'none');

  vm.runInContext("setActiveHarness('cursor')", context);
  assert.match(text('monitor-stats'), /honor observed 0 · quota available 0 held 1 unknown 1 · usage observed 0/);

  vm.runInContext("setActiveHarness('claude-code')", context);
  assert.match(text('monitor-stats'), /honor observed 1 · quota available 1 held 0 unknown 0 · usage observed 1/);

  vm.runInContext("setActiveHarness('grok')", context);
  assert.match(text('monitor-stats'), /honor unobserved · quota unknown · usage unobserved/);
  assert.equal(document.getElementById('tokens-section').style.display, 'none');
});

test('observed child tokens show sum or unobserved per tab', () => {
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
  context.fixture = {
    harnesses: ['claude-code', 'cursor'], agents: [],
    observed_child_by_harness: {
      'claude-code': { tokens: 150, usage: 1 },
    },
    switches: [
      { timestamp, harness: 'claude-code', verdict: 'OK', applied: false,
        quota: 'unknown', honor: 'unobserved', usage: 'observed' },
      { timestamp, harness: 'cursor', verdict: 'OK', applied: false,
        quota: 'unknown', honor: 'unobserved', usage: 'unobserved' },
    ],
    stats: { total: 2, down: 0, up: 0 },
  };
  const text = id => document.getElementById(id).innerHTML.replace(/<[^>]*>/g, '');
  vm.runInContext('renderMonitor(fixture)', context);
  assert.match(text('monitor-stats'), /child tokens 150 \(1 usage\)/);

  vm.runInContext("setActiveHarness('cursor')", context);
  assert.match(text('monitor-stats'), /child tokens unobserved/);

  vm.runInContext("setActiveHarness('claude-code')", context);
  assert.match(text('monitor-stats'), /child tokens 150 \(1 usage\)/);
});

test('hostile event and agent fields render inert and chips use no inline handlers', () => {
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
  const payload = '<img src=x onerror=alert(1)>';
  context.fixture = {
    harnesses: [payload], agents: [{ timestamp, model: payload, task: payload }],
    switches: [{
      timestamp, harness: payload, verdict: 'OK', complexity: payload,
      final_model: payload, final_reasoning_effort: payload,
    }],
    stats: { total: 1, down: 0, up: 0, dollars: payload,
      tokens_saved: { line: payload, by_agent_type: [{ agent_type: payload, line: payload }] } },
    compaction: { token_state: payload },
  };
  vm.runInContext('renderMonitor(fixture)', context);
  for (const id of ['monitor-status', 'monitor-switches', 'monitor-agents',
    'monitor-active-agents', 'compaction-report', 'tokens-saved']) {
    const html = document.getElementById(id).innerHTML;
    assert.doesNotMatch(html, /<img/i, id);
    assert.doesNotMatch(html, /onclick/i, id);
  }
  assert.match(document.getElementById('monitor-agents').innerHTML, /&lt;img/);
});
