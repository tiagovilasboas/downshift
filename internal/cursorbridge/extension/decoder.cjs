// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
'use strict';

const MAX_BYTES = 1 << 20;

// Only the quota subset is decoded. Unknown fields, including spend-limit data,
// are skipped; messages, headers, IDs and account information are not exported.
function fields(input) {
  const b = Buffer.from(input);
  if (b.length > MAX_BYTES) throw new Error('Response exceeds 1MiB');
  let pos = 0;
  const readVarint = () => {
    let value = 0n;
    for (let shift = 0n; shift < 70n; shift += 7n) {
      if (pos >= b.length) throw new Error('Truncated varint');
      const byte = b[pos++];
      if (shift === 63n && byte > 1) throw new Error('Invalid varint');
      value |= BigInt(byte & 127) << shift;
      if (!(byte & 128)) return value;
    }
    throw new Error('Invalid varint');
  };
  const result = [];
  while (pos < b.length) {
    const tag = Number(readVarint());
    const number = Math.floor(tag / 8), wire = tag % 8;
    if (!number || !Number.isSafeInteger(tag)) throw new Error('Invalid field');
    let value;
    if (wire === 0) {
      const raw = readVarint();
      value = raw <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(raw) : raw;
    } else if (wire === 1 || wire === 5 || wire === 2) {
      const length = wire === 2 ? Number(readVarint()) : wire === 1 ? 8 : 4;
      if (!Number.isSafeInteger(length) || length < 0 || pos + length > b.length) throw new Error('Truncated field');
      value = b.subarray(pos, pos + length); pos += length;
    } else {
      throw new Error('Unsupported wire type');
    }
    result.push({number, wire, value});
  }
  return result;
}

function decodeUsage(input, observedAt = new Date()) {
  const out = {observed_at: observedAt.toISOString(), plan_usage: {}, auto_bucket_models: []};
  let enabled = false, planFound = false;
  for (const f of fields(input)) {
    if (f.number === 2) {
      if (f.wire !== 0 || !Number.isSafeInteger(f.value) || f.value <= 0) throw new Error('Invalid billing cycle');
      out.billing_cycle_end = f.value;
    } else if (f.number === 3) {
      if (f.wire !== 2) throw new Error('Invalid plan usage');
      planFound = true;
      for (const p of fields(f.value)) {
        if (p.number !== 12 && p.number !== 13) continue;
        if (p.wire !== 1) throw new Error('Invalid quota percentage');
        const used = p.value.readDoubleLE();
        if (!Number.isFinite(used) || used < 0) throw new Error('Invalid quota percentage');
        out.plan_usage[p.number === 12 ? 'auto_percent_used' : 'api_percent_used'] = used;
      }
    } else if (f.number === 6) {
      if (f.wire !== 0 || (f.value !== 0 && f.value !== 1)) throw new Error('Invalid usage flag');
      enabled = f.value === 1;
    } else if (f.number === 13) {
      if (f.wire !== 2 || f.value.length > 256) throw new Error('Invalid model ID');
      const id = f.value.toString('utf8');
      if (!/^[A-Za-z0-9][A-Za-z0-9._:/+-]*$/.test(id)) throw new Error('Invalid model ID');
      if (!out.auto_bucket_models.includes(id)) out.auto_bucket_models.push(id);
    }
  }
  // Absence stays unknown. No provider-name or price heuristic establishes membership.
  if (!enabled || !planFound || !out.billing_cycle_end || !out.auto_bucket_models.length ||
      out.plan_usage.auto_percent_used === undefined) throw new Error('Current Cursor quota is unavailable');
  return out;
}
function decodeModels(input) {
  const ids = [];
  let count = 0;
  for (const f of fields(input)) {
    if (f.number !== 2) continue;
    if (f.wire !== 2) throw new Error('Invalid model entry');
    count++;
    let id, blocked = false;
    for (const m of fields(f.value)) {
      if (m.number === 1) {
        if (m.wire !== 2 || m.value.length > 256) throw new Error('Invalid model ID');
        id = m.value.toString('utf8');
        if (!/^[A-Za-z0-9][A-Za-z0-9._:/+-]*$/.test(id)) throw new Error('Invalid model ID');
      } else if ([5, 6, 23, 35, 43].includes(m.number)) {
        if (m.wire !== 0 || typeof m.value !== 'number') throw new Error('Invalid model status');
        if ((m.number === 5 && m.value === 0) || (m.number === 6 && m.value === 2) ||
            ([23, 35, 43].includes(m.number) && m.value !== 0)) blocked = true;
      }
    }
    if (!id) throw new Error('Missing model ID');
    if (!blocked && !ids.includes(id)) ids.push(id);
  }
  if (!count || !ids.length) throw new Error('Native model catalog is unavailable');
  return ids;
}
module.exports = {decodeUsage, decodeModels, MAX_BYTES};
