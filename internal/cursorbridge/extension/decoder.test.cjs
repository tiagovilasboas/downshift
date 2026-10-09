// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const {decodeUsage, decodeModels, MAX_BYTES} = require('./decoder.cjs');
const varint = (n) => {
  n = BigInt(n); const bytes = [];
  while (n > 127n) { bytes.push(Number(n & 127n) | 128); n >>= 7n; }
  bytes.push(Number(n)); return Buffer.from(bytes);
};
const field = (n, value, wire = 2) => Buffer.concat([varint(n * 8 + wire), wire === 2 ? varint(value.length) : Buffer.alloc(0), wire === 0 ? varint(value) : value]);
const double = (n, value) => { const b = Buffer.alloc(8); b.writeDoubleLE(value); return field(n, b, 1); };
const fixture = (auto = 60, api = 100, models = ['vendor-new-model']) => Buffer.concat([
  field(2, 1792000000000, 0), field(3, Buffer.concat([double(12, auto), double(13, api)])),
  field(6, 1, 0), ...models.map(id => field(13, Buffer.from(id))),
]);
test('exports native dynamic membership, both pools and source timestamp only', () => {
  const out = decodeUsage(Buffer.concat([fixture(), field(7, Buffer.from('private display text'))]), new Date('2026-10-09T12:00:00Z'));
  assert.deepEqual(out, {observed_at: '2026-10-09T12:00:00.000Z', billing_cycle_end: 1792000000000, plan_usage: {auto_percent_used: 60, api_percent_used: 100}, auto_bucket_models: ['vendor-new-model']});
  assert.equal(out.available_models_complete, undefined);
});
test('zero usage remains zero and over-limit remains exhausted', () => {
  assert.equal(decodeUsage(fixture(0, 120)).plan_usage.auto_percent_used, 0);
  assert.equal(decodeUsage(fixture(0, 120)).plan_usage.api_percent_used, 120);
});
test('missing pool membership, missing quota and disabled usage do not export credit', () => {
  assert.throws(() => decodeUsage(fixture(60, 100, [])));
  assert.throws(() => decodeUsage(Buffer.concat([field(2, 1792000000000, 0), field(3, double(13, 20)), field(6, 1, 0), field(13, Buffer.from('new-model'))])));
  assert.throws(() => decodeUsage(Buffer.concat([fixture(), field(6, 0, 0)])));
});
test('malformed and oversized responses, invalid percentages and unsafe IDs fail closed', () => {
  for (const bytes of [Buffer.from([0x1a, 0xff]), Buffer.from([0]), Buffer.alloc(MAX_BYTES + 1), fixture(NaN), fixture(-1), fixture(60, 20, ['x\nsecret'])]) assert.throws(() => decodeUsage(bytes));
});

test('catalog preserves provider IDs and excludes disabled, hidden, user-added and non-agent entries', () => {
  const model = (id, extra = Buffer.alloc(0)) => field(2, Buffer.concat([field(1, Buffer.from(id)), extra]));
  const catalog = Buffer.concat([model('new-provider-id'), model('hidden', field(35, 1, 0)), model('disabled', field(6, 2, 0)), model('local', field(23, 1, 0)), model('chat-only', field(5, 0, 0)), model('admin-disabled', field(43, 1, 0))]);
  assert.deepEqual(decodeModels(catalog), ['new-provider-id']);
  assert.throws(() => decodeModels(field(2, Buffer.alloc(0))));
  assert.throws(() => decodeModels(Buffer.alloc(0)));
});
