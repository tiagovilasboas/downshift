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

function shortModel(m) {
  if (!m) return 'default';
  for (const k of ['haiku','sonnet','opus','gpt-6-l','gpt-6-s','grok']) if (m.includes(k)) return k;
  return m.slice(0, 8);
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

function setActiveHarness(h) {
  activeHarness = activeHarness === h ? null : h; // toggle off if same
  if (latestData) renderMonitor(latestData);
}

function renderMonitor(data) {
  latestData = data; // always keep latest snapshot
  if (!data) {
    set('monitor-status', `<span class="dim">◯</span> ${dim('server offline — run: downshift serve')}`);
    set('monitor-switches', dim('no data'));
    set('monitor-agents',   dim('no data'));
    set('monitor-stats',    dim('—'));
    return;
  }

  const { harnesses=[], switches=[], agents=[], stats={} } = data;

  // Harness chips — clickable tabs
  const allChip = `<span class="chip${activeHarness===null?' hi':''}" onclick="setActiveHarness(null)" style="cursor:pointer">all</span>`;
  const hchips = harnesses.map(h =>
    `<span class="chip${h===activeHarness?' hi':''}" onclick="setActiveHarness('${h}')" style="cursor:pointer">${h}</span>`
  ).join('');
  set('monitor-status', `<span id="sse-indicator"></span> ${allChip}${hchips}`);

  // Filter by active harness
  const filteredSwitches = activeHarness
    ? switches.filter(e => e.harness === activeHarness)
    : switches;
  const filteredAgents = activeHarness
    ? agents.filter(a => a.session && activeHarness === 'kirocrew' ? true : false) // agents don't have harness field yet
    : agents;

  const swRows = filteredSwitches.slice(-6).map(e => {
    const v = e.verdict === 'DOWNSHIFT' ? ok(`↓ ${e.complexity.toLowerCase()} −${Math.round(e.estimated_savings*100)}%`)
            : e.verdict === 'UPSHIFT'   ? warn(`↑ ${e.complexity.toLowerCase()}`)
            : dim(`✓ ${e.complexity.toLowerCase()}`);
    return `<div class="row"><span class="t">${ageStr(e.timestamp)}</span><span class="m">${shortModel(e.final_model)}</span>${v}</div>`;
  }).join('') || dim(activeHarness ? `no switches for ${activeHarness}` : 'no switches yet');
  set('monitor-switches', swRows);

  const agRows = filteredAgents.slice(-4).map(a =>
    `<div class="row"><span class="t">${ageStr(a.timestamp)}</span><span class="m">${shortModel(a.model)}</span><span class="desc">${a.task.slice(0,44)}</span></div>`
  ).join('') || dim('no agents yet');
  set('monitor-agents', agRows);

  // Stats filtered
  const src = activeHarness ? switches.filter(e => e.harness === activeHarness) : switches;
  const down = src.filter(e => e.verdict === 'DOWNSHIFT');
  const up   = src.filter(e => e.verdict === 'UPSHIFT');
  const estUSD = down.reduce((s,e) => s + e.estimated_savings * 0.01, 0);
  set('monitor-stats',
    `${info(src.length)} spawns &nbsp; ${ok(`${down.length}↓`)} &nbsp; ${warn(`${up.length}↑`)} &nbsp; ${ok(`$${estUSD.toFixed(2)}`)} est. saved`
    + (activeHarness ? ` &nbsp; ${dim(`· ${activeHarness} only`)}` : '')
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
    renderMonitor(data); // renderMonitor sets latestData internally
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

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {});
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
