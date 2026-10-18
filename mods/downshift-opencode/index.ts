// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
//
// downshift-opencode — the OpenCode v2 project-scoped forwarding plugin for
// Downshift. On a `subagent` spawn it sends one route request to the fixed
// `downshift opencode` binary (wire v1), applies a `route` answer with
// `ctx.session.switchModel`, and forwards the post-spawn observations
// (switch_attempt / model_selected / child_context / child_done) as one
// arrival-ordered NDJSON batch to `downshift honor opencode` and
// `downshift end opencode`. Classification, policy and telemetry stay in the
// Go core; this file is the caller side of the contract documented in
// internal/adapters/opencode/opencode.go.
//
// Design rules (this file, not the core):
// - Fail-open (N1): any binary, parse or switch failure leaves the session
//   running on its original model and records one content-free code. It never
//   denies and never breaks the session.
// - Marker-gated: without `.dsspike-switch-enabled` in the project dir the
//   plugin is dormant — zero calls, zero state.
// - No network, no eval, no shell strings: child_process only, absolute bin
//   path pinned by sha256 at setup, strict argv arrays, scrubbed child env.
// - No content anywhere: logs carry allow-listed keys only (HMACs, model
//   ids, codes, timings). taskText stays in memory and only travels on the
//   binary's stdin; raw session ids never leave this process.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

const ID = "downshift-opencode"
const MARKER_NAME = ".dsspike-switch-enabled"
const ALLOWED_ENV = /^DOWNSHIFT_[A-Z_]+$/
const MAP_CAP = 256
const TASK_TEXT_CAP = 64 << 10
const STDOUT_CAP = 1 << 20
const BATCH_CAP = 64 << 10
const LINE_CAP = 8 << 10
const ROUTE_TIMEOUT_MS = 2000
const KILL_GRACE_MS = 200
const ANSWER_KEYS = new Set(["v", "action", "model", "reasonCode", "ttlMs"])
const LOG_KEYS = new Set([
  "ts", "event", "parent", "child", "agent", "action", "reason", "reasonCode",
  "from", "to", "result", "ms", "exit", "code", "by",
])

interface Intent { agent: string; taskText: string; tooLarge: boolean }
interface ChildState {
  parentRaw: string
  childH: string
  parentH: string
  current: string
  queue: string[]
  seq: number
  contextDone: boolean
  flushed: boolean
}
interface SpawnResult {
  code: number | null
  out: string
  timedOut: boolean
  tooLarge: boolean
  spawnError: boolean
}

function asStr(v: unknown): string | undefined {
  return typeof v === "string" ? v : undefined
}

function clip(v: string, cap: number): string {
  return v.length > cap ? v.slice(0, cap) : v
}

export default {
  id: ID,
  async setup(ctx: any) {
    // ---- runtime probe (imports allow-list + spawn capability) ------------
    // fs is critical: without it nothing can be resolved or logged to disk.
    let fs: any
    try {
      fs = await import("node:fs")
    } catch {
      return { active: false, code: "runtime_unavailable" }
    }
    let pathMod: any
    let urlMod: any
    let cryptoMod: any
    let cp: any
    let runtimeOk = true
    try { pathMod = await import("node:path") } catch { runtimeOk = false }
    try { urlMod = await import("node:url") } catch { runtimeOk = false }
    try { cryptoMod = await import("node:crypto") } catch { runtimeOk = false }
    try { cp = await import("node:child_process") } catch { runtimeOk = false }
    if (typeof cp?.spawn !== "function") runtimeOk = false

    // ---- resolution -------------------------------------------------------
    let projectDir = ""
    try {
      projectDir = fs.realpathSync(String(ctx?.location?.directory ?? ""))
    } catch {
      return { active: false, code: "config_invalid" }
    }
    if (!projectDir) return { active: false, code: "config_invalid" }
    let pluginDir = ""
    try {
      pluginDir = pathMod.dirname(urlMod.fileURLToPath(import.meta.url))
    } catch {
      pluginDir = projectDir + "/.opencode/plugins/" + ID
    }
    const markerPath = pathMod
      ? pathMod.join(projectDir, MARKER_NAME)
      : projectDir + "/" + MARKER_NAME

    // ---- config.json (next to this file, never committed) ------------------
    let cfg: any
    try {
      cfg = JSON.parse(fs.readFileSync(pluginDir + "/config.json", "utf8"))
    } catch (e: any) {
      const code = e?.code === "ENOENT" ? "config_missing" : "config_invalid"
      try { console.log(JSON.stringify({ event: "plugin.setup", active: false, code })) } catch {}
      return { active: false, code }
    }
    if (!cfg || typeof cfg !== "object" || Array.isArray(cfg)) {
      try { console.log(JSON.stringify({ event: "plugin.setup", active: false, code: "config_invalid" })) } catch {}
      return { active: false, code: "config_invalid" }
    }

    // From here on the failure line can (and should) land in config.logFile.
    let logFile = typeof cfg.logFile === "string" ? cfg.logFile : ""
    const setupFail = (code: string) => {
      const line = JSON.stringify({ event: "plugin.setup", active: false, code })
      let wrote = false
      if (logFile && logFile.startsWith("/")) {
        try { fs.appendFileSync(logFile, line + "\n", { mode: 0o600 }); wrote = true } catch {}
      }
      if (!wrote) { try { console.log(line) } catch {} }
      return { active: false, code }
    }

    // bin: absolute, present (realpath), owned by this uid, not group/world
    // writable, sha256-pinned to the configured checksum.
    const binCfg = typeof cfg.bin === "string" ? cfg.bin : ""
    if (!binCfg.startsWith("/")) return setupFail("bin_not_absolute")
    let bin = ""
    try { bin = fs.realpathSync(binCfg) } catch { return setupFail("bin_missing") }
    let binStat: any
    try {
      binStat = fs.statSync(bin)
      if (!binStat.isFile()) return setupFail("bin_missing")
    } catch { return setupFail("bin_missing") }
    const uid = typeof process?.getuid === "function" ? process.getuid() : -1
    if (uid < 0 || binStat.uid !== uid) return setupFail("bin_not_owned")
    if ((binStat.mode & 0o022) !== 0) return setupFail("bin_writable")
    const wantSum = (typeof cfg.binSHA256 === "string" ? cfg.binSHA256 : "").toLowerCase()
    let gotSum = ""
    if (/^[0-9a-f]{64}$/.test(wantSum) && cryptoMod) {
      try { gotSum = cryptoMod.createHash("sha256").update(fs.readFileSync(bin)).digest("hex") } catch {}
    }
    if (!wantSum || gotSum !== wantSum) return setupFail("checksum_mismatch")

    // salt: ≥64 hex chars (32 bytes of key material); HMAC output is truncated
    // to 64 hex chars, matching the wire correlation cap. Never logged.
    const saltHex = typeof cfg.salt === "string" ? cfg.salt : ""
    if (!/^[0-9a-f]{64,}$/i.test(saltHex)) return setupFail("salt_invalid")
    const salt = Buffer.from(saltHex, "hex")

    // logFile: absolute, 0600, appendable.
    if (!logFile.startsWith("/")) return setupFail("log_unavailable")
    try {
      if (fs.existsSync(logFile)) {
        fs.chmodSync(logFile, 0o600)
        if (((fs.statSync(logFile).mode as number) & 0o777) !== 0o600) return setupFail("log_unavailable")
      } else {
        fs.writeFileSync(logFile, "", { mode: 0o600, flag: "a" })
      }
      fs.appendFileSync(logFile, "")
    } catch {
      return setupFail("log_unavailable")
    }

    if (!runtimeOk) return setupFail("runtime_unavailable")

    // config.env: only DOWNSHIFT_* strings are ever forwarded to the child.
    const cfgEnv: Record<string, string> = {}
    if (cfg.env && typeof cfg.env === "object" && !Array.isArray(cfg.env)) {
      for (const k of Object.keys(cfg.env)) {
        const v = (cfg.env as Record<string, unknown>)[k]
        if (ALLOWED_ENV.test(k) && typeof v === "string" && v.length <= 4096) cfgEnv[k] = v
      }
    }

    // ---- state (this instance only) ---------------------------------------
    const intents = new Map<string, Intent>()
    const children = new Map<string, ChildState>()
    const parents = new Set<string>()
    let armed = false

    const hmac = (raw: string): string =>
      cryptoMod.createHmac("sha256", salt).update(raw).digest("hex").slice(0, 64)

    const log = (entry: Record<string, unknown>): void => {
      try {
        const line: Record<string, unknown> = { ts: new Date().toISOString() }
        if (typeof entry.event === "string") line.event = entry.event
        for (const k of Object.keys(entry)) {
          if (k !== "event" && LOG_KEYS.has(k)) line[k] = entry[k]
        }
        const s = JSON.stringify(line)
        if (s.length <= LINE_CAP) fs.appendFileSync(logFile, s + "\n", { mode: 0o600 })
      } catch {}
    }

    const markerOn = (): boolean => {
      try { return fs.existsSync(markerPath) } catch { return false }
    }
    const capSet = (s: Set<string>): void => {
      while (s.size > MAP_CAP) {
        const first = s.values().next().value
        if (first === undefined) break
        s.delete(first)
      }
    }
    const capMap = <K, V>(m: Map<K, V>): void => {
      while (m.size > MAP_CAP) {
        const first = m.keys().next().value
        if (first === undefined) break
        m.delete(first)
      }
    }

    const childEnv = (): Record<string, string> => {
      const env: Record<string, string> = {}
      const home = process?.env?.HOME
      if (typeof home === "string" && home) env.HOME = home
      const tmp = process?.env?.TMPDIR
      if (typeof tmp === "string" && tmp) env.TMPDIR = tmp
      for (const k of Object.keys(cfgEnv)) {
        if (ALLOWED_ENV.test(k)) env[k] = cfgEnv[k]
      }
      return env
    }

    // spawn: argv array, cwd projectDir, scrubbed env, stdout capped, and a
    // 2 s deadline enforced by SIGTERM → 200 ms → SIGKILL.
    const runBin = (args: string[], stdinText: string): Promise<SpawnResult> =>
      new Promise((resolve) => {
        let settled = false
        const done = (r: SpawnResult) => { if (!settled) { settled = true; resolve(r) } }
        let child: any
        try {
          child = cp.spawn(bin, args, {
            cwd: projectDir,
            env: childEnv(),
            stdio: ["pipe", "pipe", "ignore"],
          })
        } catch {
          done({ code: null, out: "", timedOut: false, tooLarge: false, spawnError: true })
          return
        }
        const chunks: Buffer[] = []
        let size = 0
        let timedOut = false
        let tooLarge = false
        let killTimer: any = null
        const ladder = () => {
          try { child.kill("SIGTERM") } catch {}
          try {
            killTimer = setTimeout(() => { try { child.kill("SIGKILL") } catch {} }, KILL_GRACE_MS)
            killTimer.unref?.()
          } catch {}
        }
        let timer: any = null
        try {
          timer = setTimeout(() => { timedOut = true; ladder() }, ROUTE_TIMEOUT_MS)
          timer.unref?.()
        } catch {}
        child.stdout?.on("data", (d: Buffer) => {
          if (tooLarge) return
          size += d.length
          if (size > STDOUT_CAP) { tooLarge = true; ladder(); return }
          chunks.push(d)
        })
        child.stdin?.on("error", () => {})
        try { child.stdin?.end(stdinText) } catch {}
        child.on("error", () => {
          try { clearTimeout(timer) } catch {}
          try { clearTimeout(killTimer) } catch {}
          done({ code: null, out: "", timedOut: false, tooLarge: false, spawnError: true })
        })
        child.on("close", (code: number | null) => {
          try { clearTimeout(timer) } catch {}
          try { clearTimeout(killTimer) } catch {}
          let out = ""
          try { out = Buffer.concat(chunks).toString("utf8") } catch {}
          done({ code, out, timedOut, tooLarge, spawnError: false })
        })
      })

    // fail-code for a call that never produced a usable answer.
    const failCode = (r: SpawnResult): string => {
      if (r.timedOut) return "timeout"
      if (r.tooLarge) return "too_large"
      if (r.spawnError) return "exit_spawn"
      if (r.code === null) return "exit_signal"
      return "exit_" + r.code
    }

    // F1-A4 strict answer parse: trim; exactly one line; JSON object; keys ⊆
    // {v,action,model,reasonCode,ttlMs}; v===1; action ∈ {allow,route};
    // reasonCode string ≤64; route requires model.id ≤128 non-empty and
    // providerID ≤64 when present. Anything else is parse_error → allow.
    const parseAnswer = (out: string): { action: string; reasonCode: string; to?: { providerID: string; id: string } } | null => {
      const t = out.trim()
      if (!t) return null
      const nl = t.indexOf("\n")
      const line = nl === -1 ? t : t.slice(0, nl)
      if (nl !== -1 && t.slice(nl + 1).trim() !== "") return null
      let obj: any
      try { obj = JSON.parse(line) } catch { return null }
      if (!obj || typeof obj !== "object" || Array.isArray(obj)) return null
      for (const k of Object.keys(obj)) if (!ANSWER_KEYS.has(k)) return null
      if (obj.v !== 1) return null
      if (obj.action !== "allow" && obj.action !== "route") return null
      if (obj.reasonCode !== undefined && (typeof obj.reasonCode !== "string" || obj.reasonCode.length > 64)) return null
      if (obj.ttlMs !== undefined && (typeof obj.ttlMs !== "number" || !Number.isFinite(obj.ttlMs) || obj.ttlMs < 0)) return null
      if (obj.model !== undefined) {
        if (!obj.model || typeof obj.model !== "object" || Array.isArray(obj.model)) return null
        for (const k of Object.keys(obj.model)) if (k !== "providerID" && k !== "id") return null
      }
      const reasonCode = typeof obj.reasonCode === "string" ? obj.reasonCode : ""
      if (obj.action === "allow") return { action: "allow", reasonCode }
      const id = asStr(obj.model?.id)
      if (!id || id.length > 128) return null
      const pid = asStr(obj.model?.providerID)
      if (pid !== undefined && pid.length > 64) return null
      return { action: "route", reasonCode, to: { providerID: pid ?? "", id } }
    }

    // Envelopes: v1 observation lines ≤8KiB; optional fields omitted when
    // absent; seq counts per child in arrival order.
    const envelope = (st: ChildState, fields: Record<string, unknown>): void => {
      try {
        const obj: Record<string, unknown> = {
          v: 1,
          kind: "observation",
          correlation: { child: st.childH, parent: st.parentH },
          ...fields,
          seq: ++st.seq,
          tsUtc: new Date().toISOString(),
        }
        const s = JSON.stringify(obj)
        if (s.length > LINE_CAP) {
          log({ event: "envelope_dropped", code: "line_too_large", child: st.childH })
          return
        }
        st.queue.push(s)
      } catch {}
    }

    const flushChild = async (st: ChildState): Promise<void> => {
      try {
        envelope(st, { event: "child_done", model: clip(st.current, 128), status: "completed" })
        let lines = st.queue
        let batch = lines.join("\n") + "\n"
        let trimmed = false
        while (batch.length > BATCH_CAP && lines.length > 1) {
          lines = lines.slice(1)
          batch = lines.join("\n") + "\n"
          trimmed = true
        }
        if (trimmed) log({ event: "flush_trimmed", code: "batch_too_large", child: st.childH })
        const h = await runBin(["honor", "opencode"], batch)
        log({ event: "flush_honor", child: st.childH, exit: h.code })
        const e = await runBin(["end", "opencode"], batch)
        log({ event: "flush_end", child: st.childH, exit: e.code })
      } catch {}
    }

    // ---- hooks -------------------------------------------------------------
    const register = async (what: string, fn: () => unknown): Promise<boolean> => {
      try {
        await fn()
        return true
      } catch {
        log({ event: "hook_error", code: "register_failed", by: what })
        return false
      }
    }

    // (1) spawn intent: remember {agent, taskText} per parent. Never logs the
    // task text or description; >64KiB text is flagged, not stored.
    if (!(await register("tool.execute.before", () => ctx.tool.hook("execute.before", (event: any) => {
        try {
          if (!armed) return
          if (event?.tool !== "subagent") return
          const sid = asStr(event?.sessionID)
          if (!sid || !markerOn()) return
          const input = event?.input && typeof event.input === "object" ? event.input : {}
          const agent = clip(asStr(input.agent) ?? "", 64)
          const taskRaw = asStr(input.prompt) ?? ""
          const tooLarge = taskRaw.length > TASK_TEXT_CAP
          intents.set(sid, { agent, taskText: tooLarge ? "" : taskRaw, tooLarge })
          capMap(intents)
          parents.add(sid)
          capSet(parents)
          log({ event: "spawn_intent", parent: hmac(sid), agent })
        } catch {}
      })
    ))) return setupFail("runtime_unavailable")

    // (2) child prompt: one route call per child, before any other await.
    if (!(await register("session.prompt", () => ctx.session.hook("prompt", async (event: any) => {
        try {
          if (!armed) return
          const sid = asStr(event?.sessionID)
          if (!sid) return
          let info: any
          try { info = await ctx.session.get({ sessionID: sid }) } catch { return }
          const parentRaw = asStr(info?.parentID)
          if (!parentRaw) return
          if (children.has(sid)) return
          const childH = hmac(sid)
          if (!markerOn()) {
            log({ event: "gated", reason: "no_marker", child: childH })
            return
          }
          const parentH = hmac(parentRaw)
          const failPre = (code: string) =>
            log({ event: "route_fail_open", action: "allow", reasonCode: "fail_open", code, ms: 0, child: childH, parent: parentH })
          const intent = intents.get(parentRaw)
          if (!intent || !intent.taskText) return failPre("no_intent")
          if (intent.tooLarge) return failPre("task_text_too_large")
          const modelObj = info?.model
          const currentId = typeof modelObj === "string" ? modelObj : asStr(modelObj?.id)
          const currentProvider = asStr(modelObj?.providerID) ?? "opencode"
          if (!currentId) return failPre("no_model")
          const req =
            JSON.stringify({
              v: 1,
              kind: "route",
              harness: { id: "opencode", version: clip(asStr(ctx?.app?.version) ?? "", 64) },
              event: "subagent_spawn_intent",
              correlation: { child: childH, parent: parentH },
              agent: intent.agent,
              model: {
                providerID: clip(currentProvider, 64),
                id: clip(currentId, 128),
                variant: clip(typeof modelObj === "object" ? asStr(modelObj?.variant) ?? "" : "", 128),
              },
              taskText: intent.taskText,
              tsUtc: new Date().toISOString(),
            }) + "\n"
          const t0 = Date.now()
          const r = await runBin(["opencode"], req)
          const ms = Date.now() - t0
          // Register the child before applying the answer: the bus event the
          // switch provokes may arrive while switchModel is awaited.
          const st: ChildState = {
            parentRaw,
            childH,
            parentH,
            current: currentId,
            queue: [],
            seq: 0,
            contextDone: false,
            flushed: false,
          }
          children.set(sid, st)
          capMap(children)
          if (r.timedOut || r.tooLarge || r.spawnError || r.code !== 0) {
            log({ event: "route_fail_open", action: "allow", reasonCode: "fail_open", code: failCode(r), ms, child: childH, parent: parentH })
            return
          }
          const ans = parseAnswer(r.out)
          if (ans === null) {
            log({ event: "route_fail_open", action: "allow", reasonCode: "fail_open", code: "parse_error", ms, child: childH, parent: parentH })
            return
          }
          log({
            event: "route_answer",
            action: ans.action,
            reasonCode: ans.reasonCode,
            ...(ans.to ? { to: clip(ans.to.id, 128) } : {}),
            ms,
            child: childH,
            parent: parentH,
          })
          if (ans.action === "route" && ans.to) {
            if (ans.to.id === currentId) {
              log({ event: "route_noop", to: clip(ans.to.id, 128), reasonCode: ans.reasonCode, child: childH })
              return
            }
            const providerID = ans.to.providerID || currentProvider
            try {
              await ctx.session.switchModel({ sessionID: sid, model: { providerID, id: ans.to.id } })
              st.current = ans.to.id
              log({ event: "switch_attempt", from: clip(currentId, 128), to: clip(ans.to.id, 128), result: "ok", by: "ctx.session.switchModel", child: childH })
              envelope(st, {
                event: "switch_attempt",
                from: clip(currentId, 128),
                to: clip(ans.to.id, 128),
                result: "ok",
                api: "ctx.session.switchModel",
              })
            } catch {
              // A failed switch is logged and nothing else: no envelope, so
              // the honor chain can never claim a switch that did not land.
              log({ event: "switch_attempt", from: clip(currentId, 128), to: clip(ans.to.id, 128), result: "error", code: "switch_failed", child: childH })
            }
          }
        } catch {}
      })
    ))) return setupFail("runtime_unavailable")

    // (3) child context: the first one fixes the dispatch-time model.
    if (!(await register("session.context", () => ctx.session.hook("context", (event: any) => {
        try {
          if (!armed) return
          const sid = asStr(event?.sessionID)
          if (!sid) return
          const st = children.get(sid)
          if (!st || st.contextDone) return
          if (!markerOn()) return
          const m = (event as any)?.model
          const id = typeof m === "string" ? m : asStr(m?.id)
          if (!id) return
          st.contextDone = true
          st.current = id
          envelope(st, { event: "child_context", model: clip(id, 128) })
          log({ event: "child_context", model: clip(id, 128), child: st.childH })
        } catch {}
      })))) return setupFail("runtime_unavailable")

    // (4) bus: allow-listed types only, scope guard armed before the loop,
    // projection only — a raw payload is never stored, logged or forwarded.
    let busIter: any = null
    if (!(await register("event.subscribe", () => { busIter = ctx.event.subscribe() }))) {
      return setupFail("runtime_unavailable")
    }

    log({ event: "plugin.setup", active: true, code: "runtime_ok", by: ID })
    armed = true
    if (busIter) {
      void (async () => {
        try {
          for await (const ev of busIter) {
            try {
              if (!armed) continue
              const type = asStr(ev?.type)
              if (type !== "session.model.selected" && type !== "session.tool.success") continue
              const sid = asStr(ev?.data?.sessionID)
              let scoped = sid !== undefined && (children.has(sid) || parents.has(sid))
              if (!scoped) {
                const dir = asStr(ev?.location?.directory)
                if (dir === undefined) continue
                let real = ""
                try { real = fs.realpathSync(dir) } catch { continue }
                if (real !== projectDir) continue
              }
              if (sid === undefined) continue
              if (type === "session.model.selected") {
                const st = children.get(sid)
                if (!st) continue
                if (!markerOn()) continue
                const m = (ev as any)?.data?.model
                const p = (ev as any)?.data?.previous
                const model = typeof m === "string" ? m : asStr(m?.id)
                if (!model) continue
                const previous = typeof p === "string" ? p : asStr(p?.id)
                st.current = model
                envelope(st, {
                  event: "model_selected",
                  model: clip(model, 128),
                  ...(previous ? { previous: clip(previous, 128) } : {}),
                })
                log({
                  event: "model_selected",
                  model: clip(model, 128),
                  ...(previous ? { previous: clip(previous, 128) } : {}),
                  child: st.childH,
                })
                continue
              }
              // session.tool.success of a tracked parent: one flush per child.
              if (!parents.has(sid)) continue
              if (!markerOn()) continue
              const data = ev?.data && typeof ev.data === "object" ? ev.data : {}
              const toolName =
                asStr(data.state?.tool) ?? asStr(data.tool) ?? asStr(data.toolName) ?? asStr(data.name)
              if (toolName !== undefined && toolName !== "subagent") continue
              for (const st of children.values()) {
                if (st.parentRaw !== sid || st.flushed) continue
                st.flushed = true
                void flushChild(st)
                break
              }
            } catch {}
          }
        } catch {
          log({ event: "hook_error", code: "bus_loop_ended", by: "event.subscribe" })
        }
      })()
    }
  },
}
