// harness-hub · app.js
// ─────────────────────────────────────────────────────────────────────────────
// Two data sources:
//   1. Local server (harness-downshift dsmon-server at localhost:7474)
//      → routing decisions, agent spawns, live stats
//   2. GitHub API (public, no auth needed for public repos)
//      → repo list, key files (CLAUDE.md, steerings, skills)
// ─────────────────────────────────────────────────────────────────────────────

const SERVER  = 'http://localhost:7474';
const GH_USER = 'tiagovilasboas';
const GH_REPOS = ['harness-downshift', 'agent-harness'];

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

// ── routing monitor (local server) ──────────────────────────────────────────

async function fetchStatus() {
  try {
    const res = await fetch(`${SERVER}/api/status`, {signal: AbortSignal.timeout(1500)});
    if (!res.ok) throw new Error(res.status);
    return await res.json();
  } catch {
    return null;
  }
}

function renderMonitor(data) {
  if (!data) {
    set('monitor-status', `<span class="dim">◯</span> ${dim('local server offline')} ${dim('— start dsmon-server from harness-downshift')}`);
    set('monitor-switches', dim('no data · run: ./dsmon-server in harness-downshift'));
    set('monitor-agents',   dim('no data'));
    set('monitor-stats',    dim('—'));
    return;
  }

  const { harnesses=[], switches=[], agents=[], stats={} } = data;

  // status row
  const hchips = harnesses.map((h,i) =>
    `<span class="${i===harnesses.length-1?'chip hi':'chip'}">${h}</span>`
  ).join('');
  set('monitor-status', `<span class="pulse">◉</span> ${hchips}`);

  // switches
  const swRows = switches.slice(-6).map(e => {
    const v = e.verdict === 'DOWNSHIFT' ? ok(`↓ ${e.complexity.toLowerCase()} −${Math.round(e.estimated_savings*100)}%`)
            : e.verdict === 'UPSHIFT'   ? warn(`↑ ${e.complexity.toLowerCase()}`)
            : dim(`✓ ${e.complexity.toLowerCase()}`);
    return `<div class="row"><span class="t">${ageStr(e.timestamp)}</span><span class="m">${shortModel(e.final_model)}</span>${v}</div>`;
  }).join('') || dim('no switches yet');
  set('monitor-switches', swRows);

  // agents
  const agRows = agents.slice(-4).map(a =>
    `<div class="row"><span class="t">${ageStr(a.timestamp)}</span><span class="m">${shortModel(a.model)}</span><span class="desc">${a.task.slice(0,44)}</span></div>`
  ).join('') || dim('no agents yet');
  set('monitor-agents', agRows);

  // stats
  set('monitor-stats',
    `${info(stats.total||0)} events &nbsp; ${ok(`${stats.down||0}↓`)} &nbsp; ${warn(`${stats.up||0}↑`)} &nbsp; ${ok(`$${(stats.est_usd||0).toFixed(2)}`)} saved`
  );
}

// ── github explorer ──────────────────────────────────────────────────────────

async function fetchRepoFile(repo, path) {
  try {
    const res = await fetch(`https://api.github.com/repos/${GH_USER}/${repo}/contents/${path}`);
    if (!res.ok) return null;
    const {content} = await res.json();
    return atob(content.replace(/\n/g,''));
  } catch { return null; }
}

async function fetchRepoInfo(repo) {
  try {
    const res = await fetch(`https://api.github.com/repos/${GH_USER}/${repo}`);
    if (!res.ok) return null;
    return await res.json();
  } catch { return null; }
}

async function renderGithub() {
  set('gh-status', dim('loading…'));
  const results = await Promise.all(GH_REPOS.map(async repo => {
    const [info, readme, claude] = await Promise.all([
      fetchRepoInfo(repo),
      fetchRepoFile(repo, 'README.md'),
      fetchRepoFile(repo, 'CLAUDE.md'),
    ]);
    return { repo, info, readme, claude };
  }));

  set('gh-status', '');
  const cards = results.map(({repo, info, readme, claude}) => {
    const desc  = info?.description || '';
    const stars = info?.stargazers_count ?? 0;
    const lang  = info?.language || '';
    const url   = `https://github.com/${GH_USER}/${repo}`;

    const files = [];
    if (claude)  files.push(`<span class="file-chip">CLAUDE.md</span>`);
    if (readme)  files.push(`<span class="file-chip">README.md</span>`);

    return `
<div class="gh-card">
  <div class="gh-top">
    <a class="gh-name" href="${url}" target="_blank" rel="noopener noreferrer">${repo}</a>
    <span class="gh-meta">${stars ? `★ ${stars}` : ''} ${lang ? `· ${lang}` : ''}</span>
  </div>
  ${desc ? `<div class="gh-desc">${desc}</div>` : ''}
  <div class="gh-files">${files.join('')}</div>
  ${claude ? `<details class="gh-detail"><summary>CLAUDE.md</summary><pre>${claude.slice(0,800)}…</pre></details>` : ''}
</div>`;
  }).join('');

  set('gh-repos', cards || dim('no repos found'));
}

// ── init ─────────────────────────────────────────────────────────────────────

async function tick() {
  const data = await fetchStatus();
  renderMonitor(data);
}

// Register service worker
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {});
}

// First paint
tick();
renderGithub();

// Refresh monitor every 2s
setInterval(tick, 2000);

// Refresh timestamp in header
setInterval(() => {
  const el = $('clock');
  if (el) el.textContent = new Date().toLocaleTimeString();
}, 1000);
