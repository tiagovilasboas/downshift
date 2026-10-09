// Live report sensor. Reads the running dashboard and the local event log.
// Does not write production code and does not invent events.
// Run: node --experimental-websocket docs/evidence/report-e2e/live-report.check.mjs
import { spawn } from 'node:child_process';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { setTimeout as sleep } from 'node:timers/promises';
import { createInterface } from 'node:readline';
import { createReadStream } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

const BASE = 'http://127.0.0.1:7474';
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PORT = 9334;
const OUT = new URL('.', import.meta.url).pathname;
const PROFILE = '/tmp/downshift-report-e2e-chrome';
const SENTENCE = 'Bytes medidos na saída da ferramenta. Tokens indisponíveis. Não soma na economia estimada do roteamento.';
const CHIPS = ['antigravity', 'codex', 'cursor', 'claude-code', 'kirocrew', 'grok'];

function fail(findings, severity, title, detail) {
  findings.push({ severity, title, detail });
}

async function apiStatus() {
  const res = await fetch(`${BASE}/api/status`);
  const raw = await res.text();
  return { status: res.status, raw, json: JSON.parse(raw) };
}

async function countLog() {
  const path = join(homedir(), '.harness-downshift', 'events.jsonl');
  const compaction = join(homedir(), '.harness-downshift', 'context-compactions.jsonl');
  const cut = Date.now() - 24 * 60 * 60 * 1000;
  const by = {};
  const effort = [];
  let kiroLast = null;
  const rl = createInterface({ input: createReadStream(path), crlfDelay: Infinity });
  for await (const line of rl) {
    if (!line.trim()) continue;
    const e = JSON.parse(line);
    const t = Date.parse(e.timestamp);
    if (e.harness === 'kirocrew') kiroLast = e.timestamp;
    if (!Number.isFinite(t) || t < cut) continue;
    by[e.harness || ''] = (by[e.harness || ''] || 0) + 1;
    const effortValue = e.final_reasoning_effort;
    if (effortValue && effortValue !== 'unknown' && effort.length < 6) {
      effort.push({
        timestamp: e.timestamp,
        harness: e.harness,
        final_model: e.final_model,
        final_reasoning_effort: effortValue,
      });
    }
  }
  let compactionExists = false;
  try {
    const { statSync } = await import('node:fs');
    const st = statSync(compaction);
    compactionExists = st.isFile();
  } catch {
    compactionExists = false;
  }
  return { by, kiroLast, effort, compactionExists, compactionPath: compaction, eventsPath: path };
}

function startChrome() {
  rmSync(PROFILE, { recursive: true, force: true });
  const child = spawn(CHROME, [
    '--headless=new',
    '--disable-gpu',
    '--no-first-run',
    '--no-default-browser-check',
    `--remote-debugging-port=${PORT}`,
    `--user-data-dir=${PROFILE}`,
    '--window-size=1100,1600',
    '--hide-scrollbars',
    `${BASE}/?e2e=${Date.now()}`,
  ], { stdio: 'ignore' });
  return child;
}

async function waitForPage() {
  for (let i = 0; i < 40; i++) {
    try {
      const list = await fetch(`http://127.0.0.1:${PORT}/json/list`).then((r) => r.json());
      const page = list.find((t) => t.type === 'page' && t.webSocketDebuggerUrl);
      if (page) return page;
    } catch {
      // Chrome is still binding the port.
    }
    await sleep(150);
  }
  throw new Error('chrome debugging port did not expose a page');
}

function cdp(ws) {
  let seq = 0;
  const pending = new Map();
  ws.addEventListener('message', (ev) => {
    const msg = JSON.parse(ev.data);
    if (!msg.id || !pending.has(msg.id)) return;
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    if (msg.error) reject(new Error(JSON.stringify(msg.error)));
    else resolve(msg.result);
  });
  return (method, params = {}) => {
    const id = ++seq;
    ws.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
  };
}

async function evalJs(send, expression) {
  const result = await send('Runtime.evaluate', {
    expression,
    returnByValue: true,
    awaitPromise: true,
  });
  if (result.exceptionDetails) {
    throw new Error(JSON.stringify(result.exceptionDetails));
  }
  return result.result.value;
}

async function shot(send, file) {
  const height = await evalJs(send, 'Math.max(document.documentElement.scrollHeight, document.body.scrollHeight, 900)');
  await send('Emulation.setDeviceMetricsOverride', {
    width: 1100,
    height: Math.ceil(height) + 20,
    deviceScaleFactor: 2,
    mobile: false,
  });
  const png = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
  writeFileSync(file, Buffer.from(png.data, 'base64'));
}

const SNAPSHOT = `(() => {
  const text = (id) => document.getElementById(id)?.innerText || '';
  const economy = document.getElementById('economy-section');
  return {
    chips: [...document.querySelectorAll('#monitor-status .chip')].map((el) => el.textContent),
    hi: [...document.querySelectorAll('#monitor-status .chip.hi')].map((el) => el.textContent),
    signal: text('spawn-signal'),
    active: text('monitor-active-agents'),
    switches: text('monitor-switches'),
    agents: text('monitor-agents'),
    stats: text('monitor-stats'),
    economy: text('economy-bar'),
    economyVisible: !!economy && economy.style.display !== 'none',
    compaction: text('compaction-report'),
  };
})()`;

function expectFromApi(data, harness) {
  const switches = Array.isArray(data.switches) ? data.switches : [];
  const src = harness ? switches.filter((e) => e.harness === harness) : switches;
  const stats = data.stats || {};
  const down = src.filter((e) => e.verdict === 'DOWNSHIFT');
  const up = src.filter((e) => e.verdict === 'UPSHIFT');
  const totalN = harness ? src.length : (stats.total ?? src.length);
  const downN = harness ? down.length : (stats.down ?? down.length);
  const upN = harness ? up.length : (stats.up ?? up.length);
  const est = harness
    ? down.reduce((s, e) => s + (e.estimated_savings || 0) * 0.01, 0)
    : (stats.est_usd ?? 0);
  const five = Date.now() - 5 * 60 * 1000;
  const recent = src.filter((e) => Date.parse(e.timestamp) > five);
  const shown = src.slice(-6).reverse();
  return { totalN, downN, upN, est, recent, shown, src };
}

function assertTab(findings, name, dom, data) {
  const harness = name === 'all' ? null : name;
  const exp = expectFromApi(data, harness);
  if (!dom.chips.includes('all') || CHIPS.some((h) => !dom.chips.includes(h))) {
    fail(findings, 'blocker', `${name}: chips`, `chips=${dom.chips.join(',')}`);
  }
  if (harness && !dom.hi.includes(harness)) {
    fail(findings, 'blocker', `${name}: chip not selected`, `hi=${dom.hi.join(',')}`);
  }
  if (!harness && !dom.hi.includes('all')) {
    fail(findings, 'blocker', 'all: chip not selected', `hi=${dom.hi.join(',')}`);
  }
  const stats = dom.stats.replace(/\s+/g, ' ');
  if (!stats.includes(`${exp.totalN} events`) || !stats.includes(`${exp.downN}↓`) || !stats.includes(`${exp.upN}↑`)) {
    fail(findings, 'blocker', `${name}: stats`, `dom=${stats} expected ${exp.totalN} events ${exp.downN}↓ ${exp.upN}↑`);
  }
  if (harness && !stats.includes(`${harness} only`)) {
    fail(findings, 'blocker', `${name}: harness scope`, stats);
  }
  if (exp.shown.length === 0) {
    const empty = `no ${harness} events in the last 24h`;
    if (!dom.switches.includes(empty)) {
      fail(findings, 'blocker', `${name}: empty switches`, `dom=${dom.switches}`);
    }
  } else {
    if (harness && dom.agents.includes(`no ${harness} events in the last 24h`)) {
      fail(findings, 'blocker', `${name}: agents block denies events listed above`, dom.agents);
    }
    for (const e of exp.shown) {
      const id = e.final_model || e.requested_model || e.model || 'unknown';
      if (!dom.switches.includes(id)) {
        fail(findings, 'blocker', `${name}: model id missing`, `want ${id} in ${dom.switches}`);
      }
      const effort = e.final_reasoning_effort;
      if (effort && effort !== 'unknown' && !dom.switches.includes(effort)) {
        fail(findings, 'blocker', `${name}: effort missing`, `${id} effort ${effort} not in ${dom.switches}`);
      }
      if (effort === 'unknown' && dom.switches.includes('· unknown')) {
        fail(findings, 'major', `${name}: fake effort`, dom.switches);
      }
    }
  }
  if (exp.recent.length === 0) {
    if (!dom.active.includes('none in the last 5 min')) {
      fail(findings, 'blocker', `${name}: active agents`, dom.active);
    }
  } else {
    for (const e of exp.recent) {
      const id = e.final_model || e.requested_model || 'unknown';
      if (!dom.active.includes(id)) {
        fail(findings, 'blocker', `${name}: recent model`, `want ${id} in ${dom.active}`);
      }
    }
  }
  if (exp.downN > 0) {
    if (!dom.economyVisible) fail(findings, 'blocker', `${name}: savings hidden`, dom.economy);
    const shownUsd = `$${exp.est.toFixed(2)}`;
    if (!dom.economy.includes(shownUsd)) {
      fail(findings, 'major', `${name}: savings text`, `dom=${dom.economy} expected ${shownUsd} from est=${exp.est}`);
    }
    if (dom.economy.includes('bytes')) {
      fail(findings, 'blocker', `${name}: savings mixed with compaction`, dom.economy);
    }
  } else if (dom.economyVisible) {
    fail(findings, 'major', `${name}: savings shown with zero downshifts`, dom.economy);
  }
  if (!dom.compaction.includes(SENTENCE)) {
    fail(findings, 'blocker', `${name}: compaction sentence`, dom.compaction);
  }
  return exp;
}

async function main() {
  mkdirSync(OUT, { recursive: true });
  const findings = [];
  const api = await apiStatus();
  writeFileSync(join(OUT, 'api-status.json'), api.raw);
  if (api.status !== 200) fail(findings, 'blocker', 'api status', String(api.status));
  const agentsRaw = api.raw.match(/"agents"\s*:\s*(\[|null)/);
  if (!agentsRaw || agentsRaw[1] !== '[') {
    fail(findings, 'blocker', 'agents is not a JSON array', agentsRaw ? agentsRaw[0] : 'missing');
  }
  const log = await countLog();
  const chrome = startChrome();
  let ws;
  try {
    const page = await waitForPage();
    ws = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => {
      ws.addEventListener('open', resolve);
      ws.addEventListener('error', () => reject(new Error('cdp socket failed')));
    });
    const send = cdp(ws);
    await send('Page.enable');
    await send('Runtime.enable');
    await send('Page.navigate', { url: `${BASE}/?e2e=${Date.now()}` });
    let ready = '';
    for (let i = 0; i < 40; i++) {
      ready = await evalJs(send, `document.getElementById('monitor-status')?.innerText || ''`);
      if (ready.includes('kirocrew') && ready.includes('grok')) break;
      await sleep(200);
    }
    if (!ready.includes('kirocrew')) fail(findings, 'blocker', 'page did not render chips', ready);

    const tabs = ['all', ...CHIPS];
    const doms = {};
    for (const tab of tabs) {
      if (tab !== 'all') {
        await evalJs(send, `setActiveHarness(${JSON.stringify(tab)})`);
      }
      await sleep(100);
      const dom = await evalJs(send, SNAPSHOT);
      doms[tab] = dom;
      await shot(send, join(OUT, `${tab}.png`));
      assertTab(findings, tab, dom, api.json);
    }
    writeFileSync(join(OUT, 'dom.json'), JSON.stringify(doms, null, 2));

    const data = api.json;
    const switches = data.switches || [];
    const harnessesInPayload = new Set(switches.map((e) => e.harness));
    for (const name of ['antigravity', 'codex', 'cursor', 'claude-code', 'grok']) {
      const inLog = log.by[name] || 0;
      const inApi = [...harnessesInPayload].includes(name);
      if (inLog > 0 && !inApi) {
        fail(findings, 'blocker', `${name}: 24h events missing from the tab`,
          `events.jsonl has ${inLog} events in the last 24h, /api/status switches has ${switches.length} rows and none for ${name}. The tab says "no ${name} events in the last 24h".`);
      }
      if (inLog > switches.filter((e) => e.harness === name).length) {
        fail(findings, 'blocker', `${name}: tab is the API tail, not the 24h window`,
          `events.jsonl has ${inLog} events in the last 24h; /api/status switches includes ${switches.filter((e) => e.harness === name).length}.`);
      }
    }
    if ((log.by.kirocrew || 0) > 0) {
      fail(findings, 'blocker', 'kirocrew invented window', `log count ${log.by.kirocrew}`);
    }
    const kiroDom = doms.kirocrew;
    if (kiroDom && /grok-|gpt-|claude-|flash|muse-/.test(kiroDom.switches + kiroDom.active)) {
      fail(findings, 'blocker', 'kirocrew shows another harness', kiroDom.switches);
    }
    const compaction = data.compaction || {};
    if (!log.compactionExists) {
      const names = Object.keys(compaction.by_harness || {});
      if (names.length > 0) {
        fail(findings, 'blocker', 'compaction invented harness rows', names.join(','));
      }
    } else if (Object.keys(compaction.by_harness || {}).length === 0 && compaction.by_harness_state === 'observed') {
      fail(findings, 'major', 'compaction log exists but by_harness is empty', log.compactionPath);
    }
    writeFileSync(join(OUT, 'log-summary.json'), JSON.stringify({
      by: log.by,
      kiroLast: log.kiroLast,
      effortSamples: log.effort,
      compactionExists: log.compactionExists,
      capturedAt: new Date().toISOString(),
    }, null, 2));
  } finally {
    if (ws) ws.close();
    chrome.kill('SIGKILL');
    await sleep(200);
    try {
      rmSync(PROFILE, { recursive: true, force: true, maxRetries: 3 });
    } catch {
      // A leftover Chrome profile must not hide the verdict.
    }
  }

  const blockers = findings.filter((f) => f.severity === 'blocker');
  const verdict = blockers.length ? 'BLOCK' : 'PASS';
  writeFileSync(join(OUT, 'findings.json'), JSON.stringify({ verdict, findings }, null, 2));
  console.log(JSON.stringify({ verdict, findings }, null, 2));
  process.exit(blockers.length ? 1 : 0);
}

main().catch((err) => {
  console.error(err);
  process.exit(2);
});
