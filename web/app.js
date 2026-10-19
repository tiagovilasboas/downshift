// harness-hub · app.js
// downshift web dashboard · app.js
// ─────────────────────────────────────────────────────────────────────────────
// Data source: local server (/api/status, /events SSE) — relative URLs.
// Shows routing decisions only when subagents are dispatched via spawn_run
// and the downshift hook is active. Not total token consumption.
// ─────────────────────────────────────────────────────────────────────────────

// ── helpers ──────────────────────────────────────────────────────────────────

const $ = id => document.getElementById(id);
const set = (id, html) => { const el = $(id); if (el) el.innerHTML = html; };
const dim  = s => `<span class="dim">${s}</span>`;
const ok   = s => `<span class="ok">${s}</span>`;
const warn = s => `<span class="warn">${s}</span>`;
const info = s => `<span class="info">${s}</span>`;

function ageStr(ts) {
  const s = (Date.now() - new Date(ts).getTime()) / 1000;
  if (s < 60)   return `${Math.floor(s)}s`;
  if (s < 3600) return `${Math.floor(s/60)}m`;
  return new Date(ts).toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'});
}

// spawnSurfaceLine is the same three states for every harness tab.
// An empty tab is unobserved / unknown. Counts come from the events in view.
// quota unknown or held does not count as available. honor unobserved does
// not count as observed.
// observedChildLine reports the end-of-day child token sum for all harnesses
// or one tab. usage == 0 means unobserved, never a measured zero.
function observedChildLine(data, harness) {
  const map = (data && data.observed_child_by_harness) || {};
  if (harness) {
    const row = map[harness];
    if (!row || !row.usage) return 'child tokens unobserved';
    return `child tokens ${escapeHtml(row.tokens)} (${escapeHtml(row.usage)} usage)`;
  }
  let tokens = 0;
  let usage = 0;
  for (const row of Object.values(map)) {
    if (!row || !row.usage) continue;
    tokens += row.tokens || 0;
    usage += row.usage;
  }
  if (usage === 0) return 'child tokens unobserved';
  return `child tokens ${escapeHtml(tokens)} (${escapeHtml(usage)} usage)`;
}

function spawnSurfaceLine(events) {
  if (!events.length) {
    return 'honor unobserved · quota unknown · usage unobserved';
  }
  let honorObs = 0, usageObs = 0, available = 0, held = 0, unknown = 0;
  for (const e of events) {
    if (e.honor === 'observed') honorObs++;
    if (e.usage === 'observed') usageObs++;
    if (e.quota === 'available') available++;
    else if (e.quota === 'held') held++;
    else unknown++;
  }
  return `honor observed ${honorObs} · quota available ${available} held ${held} unknown ${unknown} · usage observed ${usageObs}`;
}

function modelVersion(e) {
  const id = escapeHtml((e && (e.final_model || e.requested_model || e.model)) || 'unknown');
  const effort = e && (e.final_reasoning_effort || e.requested_reasoning_effort);
  if (effort && effort !== 'unknown') return `${id} · ${escapeHtml(effort)}`;
  return id;
}

// ── routing monitor ──────────────────────────────────────────────────────────

async function fetchStatus() {
  try {
    // Use RELATIVE url — always same origin as the page
    const res = await fetch('/api/status', {signal: AbortSignal.timeout(1500)});
    if (!res.ok) throw new Error(res.status);
    return await res.json();
  } catch {
    return null;
  }
}

// ── harness tab filter ────────────────────────────────────────────────────────

let activeHarness = null; // null = show all
let _justUpdated  = false; // true briefly after SSE update → triggers flash CSS

function setActiveHarness(h) {
  activeHarness = activeHarness === h ? null : h; // toggle off if same
  if (latestData) renderMonitor(latestData);
}

function renderMonitor(data) {
  latestData = data; // always keep latest snapshot
  if (!data) {
    set('monitor-status', `<span class="dim">○</span> ${dim('server offline — run: downshift serve')}`);
    set('monitor-switches', dim('no data'));
    set('monitor-agents',   dim('no data'));
    set('monitor-stats',    dim('—'));
    return;
  }

  // The server encodes a nil slice as null. A default in destructuring does not
  // replace null, and agents.filter then throws after the chips are painted.
  const harnesses = Array.isArray(data.harnesses) ? data.harnesses : [];
  const switches = Array.isArray(data.switches) ? data.switches : [];
  const agents = Array.isArray(data.agents)
    ? data.agents.filter(a => a && typeof a === 'object')
    : [];
  const stats = data.stats && typeof data.stats === 'object' ? data.stats : {};

  // Harness chips — clickable tabs. KiroCrew stays even with an empty 24h window.
  const chipNames = ['antigravity', 'codex', 'cursor', 'claude-code', 'kirocrew', 'grok'];
  for (const h of harnesses) {
    if (h && !chipNames.includes(h)) chipNames.push(h);
  }
  const allChip = `<span class="chip${activeHarness===null?' hi':''}">all</span>`;
  const hchips = chipNames.map(h =>
    `<span class="chip${h===activeHarness?' hi':''}" data-harness="${escapeHtml(h)}">${escapeHtml(h)}</span>`
  ).join('');
  set('monitor-status', allChip + hchips);

  // Filter by active harness
  const filteredSwitches = activeHarness
    ? switches.filter(e => e.harness === activeHarness)
    : switches;
  const filteredAgents = activeHarness
    ? agents.filter(a => a.session && activeHarness === 'kirocrew' ? true : false) // agents don't have harness field yet
    : agents;

  // Spawns live on the routing events. agents.jsonl is often empty, so the
  // 5-minute list must use switches or it stays "none" while a spawn is on screen.
  const fiveMinAgo = Date.now() - 5 * 60 * 1000;
  const recent = (ts) => {
    const t = new Date(ts).getTime();
    return Number.isFinite(t) && t > fiveMinAgo;
  };
  const activeSwitches = filteredSwitches.filter(e => recent(e.timestamp));
  // agents.jsonl is outside the routing window and has no harness. The
  // 5-minute list is switches only.
  const signal = document.getElementById('spawn-signal');
  if (signal) {
    const n = activeSwitches.length;
    if (n > 0) {
      signal.className = 'spawn-signal';
      signal.textContent = `active agent spawned · ${n}`;
    } else {
      signal.className = 'spawn-signal off';
      signal.textContent = 'none in the last 5 min';
    }
  }
  if (activeSwitches.length === 0) {
    set('monitor-active-agents', dim('none in the last 5 min'));
  } else {
    const fromSwitches = activeSwitches.slice().reverse().map(e => {
      const complexity = escapeHtml((e.complexity || 'unknown').toLowerCase());
      const verdict = e.verdict === 'DOWNSHIFT' ? `↓ ${complexity}`
        : e.verdict === 'UPSHIFT' ? `↑ ${complexity}`
        : e.verdict === 'UNKNOWN' ? `? ${complexity}`
        : `✓ ${complexity}`;
      const effort = e.final_reasoning_effort && e.final_reasoning_effort !== 'unknown'
        ? ` · ${escapeHtml(e.final_reasoning_effort)}` : '';
      return `<div class="row">
        <span class="t">${ageStr(e.timestamp)}</span>
        <span class="m"><span class="active-dot">●</span>${modelVersion(e)}</span>
        <span class="tag-spawned">spawned</span>
        <span class="desc">${escapeHtml(e.harness || '')} · ${verdict}${effort}</span>
      </div>`;
    });
    set('monitor-active-agents', fromSwitches.join(''));
  }

  // ── switches (last 6, newest first with flash) ──
  const swRows = filteredSwitches.slice(-6).reverse().map((e, i) => {
    const complexity = escapeHtml((e.complexity || 'unknown').toLowerCase());
    const v = e.verdict === 'DOWNSHIFT' ? ok(`↓ ${complexity}`)
            : e.verdict === 'UPSHIFT'   ? warn(`↑ ${complexity}`)
            : e.verdict === 'UNKNOWN'   ? dim(`? ${complexity}`)
            : dim(`✓ ${complexity}`);
    const isNew = i === 0 && _justUpdated;
    return `<div class="row${isNew?' new':''}"><span class="t">${ageStr(e.timestamp)}</span><span class="m">${modelVersion(e)}</span><span class="tag-spawned">spawned</span>${v}</div>`;
  }).join('') || dim(activeHarness ? `no ${escapeHtml(activeHarness)} events in the last 24h` : 'no active agent spawned');
  set('monitor-switches', swRows);

  const agRows = filteredAgents.slice(-4).map(a =>
    `<div class="row"><span class="t">${ageStr(a.timestamp)}</span><span class="m">${escapeHtml(a.model || 'unknown')}</span><span class="tag-spawned">spawned</span><span class="desc">${escapeHtml(String(a.task || '').slice(0,44))}</span></div>`
  ).join('') || dim('no spawned agents in the agent log');
  set('monitor-agents', agRows);

  // ── stats: same rule for all and for one harness chip ──
  const src = activeHarness ? switches.filter(e => e.harness === activeHarness) : switches;
  const down = src.filter(e => e.verdict === 'DOWNSHIFT' && e.applied === true);
  const up   = src.filter(e => e.verdict === 'UPSHIFT' && e.applied === true);
  const totalN = activeHarness ? src.length : (stats.total ?? src.length);
  const downN = activeHarness ? down.length : (stats.down ?? down.length);
  const upN = activeHarness ? up.length : (stats.up ?? up.length);
  set('monitor-stats',
    `${info(escapeHtml(totalN))} events &nbsp; ${ok(`${escapeHtml(downN)}↓`)} &nbsp; ${warn(`${escapeHtml(upN)}↑`)}` +
    (activeHarness ? ` &nbsp; ${dim(`· ${escapeHtml(activeHarness)} only`)}` : '') +
    `<div class="dim">${spawnSurfaceLine(src)}</div>` +
    `<div class="dim">${observedChildLine(data, activeHarness)}</div>`
  );

  // ── tokens saved: the server's TokenSavings, never dollars ──
  renderTokensSaved(stats, activeHarness);
  renderCompaction(data.compaction);
}

// tokensBlockFor is the server's tokens saved block for all harnesses or one
// chip. A harness with no priced child token record has no block.
function tokensBlockFor(stats, harness) {
  if (!harness) return stats.tokens_saved;
  const map = stats.tokens_saved_by_harness;
  return map && typeof map === 'object' ? map[harness] : undefined;
}

// renderTokensSaved draws the share of child tokens that ran on a cheaper
// model than requested, work and cache apart, with one line per agent type.
// No figure until a child's tokens are known; dollars live in the report.
function renderTokensSaved(stats, harness) {
  const block = tokensBlockFor(stats, harness);
  const sec = document.getElementById('tokens-section');
  if (!block || !block.line) {
    if (sec) sec.style.display = 'none';
    set('tokens-saved', '');
    return;
  }
  if (sec) sec.style.display = '';
  const types = Array.isArray(block.by_agent_type) ? block.by_agent_type : [];
  const typeRows = types.length > 1 || (types.length === 1 && types[0].agent_type !== 'untyped')
    ? types.map(t => `<div class="dim">${escapeHtml(t.line || '')}</div>`).join('') : '';
  const dollars = escapeHtml(stats.dollars || 'downshift report');
  set('tokens-saved', `
<div class="tokens-bar">
  <div>
    <div class="big">${escapeHtml(block.line)}</div>
    <div class="sub">ran on a cheaper model than requested · last 24h · dollars: ${dollars}</div>
    ${typeRows}
  </div>
</div>`);
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, ch => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  }[ch]));
}

function renderCompaction(c) {
  const before = (c && c.bytes_before) || 0;
  const after = (c && c.bytes_after) || 0;
  const reduced = (c && c.bytes_reduced) || 0;
  const pct = c && typeof c.savings_pct === 'number' ? c.savings_pct : 0;
  const tokenState = (c && c.token_state) || 'unavailable';
  const harnessState = c && c.by_harness_state;
  let harnessRows = '';
  if (harnessState === 'unavailable') {
    harnessRows = `<div class="dim">by_harness unavailable</div>`;
  } else if (c && c.by_harness && typeof c.by_harness === 'object') {
    harnessRows = Object.keys(c.by_harness).sort().map(name => {
      const row = c.by_harness[name] || {};
      const bin = Number(row.bytes_before) || 0;
      const aft = Number(row.bytes_after) || 0;
      const red = Number(row.bytes_reduced) || 0;
      return `<div class="row"><span class="m">${escapeHtml(name)}</span> ${bin} bytes in · ${aft} bytes after · ${red} bytes reduced</div>`;
    }).join('');
  }
  set('compaction-report',
    `<div class="stats-line">${escapeHtml(before)} bytes in · ${escapeHtml(after)} bytes after · ${ok(escapeHtml(reduced) + ' bytes reduced')} · savings_pct ${pct.toFixed(1)}</div>` +
    `<div class="dim">token_state ${escapeHtml(tokenState)}</div>` +
    harnessRows +
    `<div>Bytes medidos na saída da ferramenta. Tokens indisponíveis. Não soma na economia estimada do roteamento.</div>`
  );
}

// ── SSE: push updates from server ────────────────────────────────────────────
// Server sends 'data: {"type":"update"}' whenever events.jsonl or agents.jsonl
// changes. We re-fetch /api/status on each event for the full snapshot.

let sseConnected = false;

function connectSSE() {
  const es = new EventSource('/events');

  es.addEventListener('update', async () => {
    const data = await fetchStatus();
    _justUpdated = true;
    renderMonitor(data); // renderMonitor sets latestData internally
    setTimeout(() => { _justUpdated = false; }, 1000);
  });

  es.addEventListener('open', () => {
    sseConnected = true;
    set('sse-indicator', `<span class="pulse">◉</span>`);
  });

  es.addEventListener('error', () => {
    sseConnected = false;
    set('sse-indicator', `<span class="dim">○</span>`);
    es.close();
    setTimeout(connectSSE, 3000);
  });
}

// ── age counters update every second ─────────────────────────────────────────
// renderMonitor() stores latestData internally; the 1s ticker re-renders it
// so age strings tick ("5s" → "6s") without a new fetch.

let latestData = null;

function updateTimestamps() {
  if (!latestData) return;
  renderMonitor(latestData);
}

// ── init ─────────────────────────────────────────────────────────────────────

// Do not register a service worker. A previous one cached a broken app.js
// and a stale /api/status. sw.js now only unregisters that worker.

// Chips are delegated: no inline handlers, so a CSP can forbid them.
const chipBar = $('monitor-status');
if (chipBar && chipBar.addEventListener) {
  chipBar.addEventListener('click', ev => {
    const chip = ev.target && ev.target.closest ? ev.target.closest('.chip') : null;
    if (chip) setActiveHarness(chip.dataset.harness || null);
  });
}

// First paint
fetchStatus().then(data => { latestData = data; renderMonitor(data); });

// Connect SSE for push updates
connectSSE();

// Age counter: re-render timestamps every second without re-fetching
setInterval(() => {
  if (latestData) renderMonitor(latestData);
  const el = $('clock');
  if (el) el.textContent = new Date().toLocaleTimeString();
}, 1000);

// Fallback polling every 10s if SSE disconnected
setInterval(async () => {
  if (!sseConnected) {
    const data = await fetchStatus();
    if (data) { latestData = data; renderMonitor(data); }
  }
}, 10000);
