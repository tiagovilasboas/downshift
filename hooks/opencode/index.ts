// Downshift's OpenCode bridge (wire v1).
//
// Written by `downshift setup`, which rewrites it. Edit nothing here.
//
// This file holds no routing policy: it forwards what OpenCode observes to
// `downshift`, and applies the answer it gets back. Classification, tier
// choice, guardrails, shell policy and telemetry live in the shared Go layers
// and serve every harness the same way.
//
// Calls the bridge makes:
//
//	downshift opencode          one route, retry or shell answer (JSON on stdout)
//	downshift honor opencode    one observation batch (resolved-model chain)
//	downshift end opencode      one observation batch (child end and usage)
//
// Every call is fail-open: a missing binary, a timeout, a non-zero exit or a
// payload the bridge cannot parse leaves OpenCode's own behavior untouched.
// The main session is never routed; only a session with a parent is.

import { Plugin } from "@opencode/plugin"
import { type ModelRef, knownModels, modelName, modelRef, resolveTarget } from "./models.ts"
import { acceptShellRewrite, resolveBin } from "./shell_trust.ts"
import { createShellSessions } from "./shell_sessions.ts"
import {
  HARNESS,
  SHELL_TIMEOUT_MS,
  WIRE_VERSION,
  answer,
  batchAnswer,
  count,
  opaque,
  taskText,
} from "./wire.ts"

type Usage = {
  input: number
  output: number
  reasoning: number
  cache: { read: number; write: number }
}

type Child = {
  sessionID: string
  child: string
  parent: string
  agent: string
  directory: string
  requested: ModelRef
  target: ModelRef | null
  switchResult: "ok" | "error" | ""
  contextModel: string
  fallback: boolean
  usage: Usage
  steps: Set<string>
  finished: boolean
}

export default Plugin.define({
  id: "downshift",
  async setup(ctx) {
    const pinnedEnv = process.env.DOWNSHIFT_BIN
    const resolved = resolveBin(pinnedEnv)
    if (resolved === null) {
      console.error(
        pinnedEnv
          ? "downshift: DOWNSHIFT_BIN must be an absolute path to an existing file; the OpenCode bridge is off"
          : "downshift: `downshift` was not found on PATH (set DOWNSHIFT_BIN to its absolute path); the OpenCode bridge is off",
      )
      return () => {}
    }
    const bin = resolved
    const version = typeof (ctx as any).app?.version === "string" ? (ctx as any).app.version : ""
    const children = new Map<string, Child>()
    const shellSessions = createShellSessions()
    let closed = false

    const base = (kind: string, event: string, child: string, parent: string) => ({
      v: WIRE_VERSION,
      kind,
      event,
      harness: { id: HARNESS, version },
      correlation: { child, parent },
      tsUtc: new Date().toISOString(),
    })

    const remember = (child: Child) => {
      children.set(child.sessionID, child)
      if (children.size > 500) {
        const oldest = children.keys().next()
        if (!oldest.done) children.delete(oldest.value)
      }
    }

    const finish = (child: Child, status: "completed" | "failed") => {
      if (child.finished) return
      child.finished = true
      children.delete(child.sessionID)
      const lines: string[] = []
      let seq = 1
      const push = (event: string, extra: Record<string, unknown>) => {
        lines.push(
          JSON.stringify({
            v: WIRE_VERSION,
            kind: "observation",
            correlation: { child: child.child, parent: child.parent },
            event,
            seq: seq++,
            tsUtc: new Date().toISOString(),
            ...extra,
          }),
        )
      }
      if (child.target) {
        push("switch_attempt", {
          from: modelName(child.requested),
          to: modelName(child.target),
          result: child.switchResult,
        })
      }
      if (child.target && child.switchResult === "ok") {
        push("model_selected", { model: modelName(child.target), previous: modelName(child.requested) })
      }
      // The last model the agent loop reported is what the child really ran
      // on. When a fallback undid the switch, this is the requested model, and
      // the shared recorder refutes the decision instead of claiming it held.
      const resolved = child.contextModel || (child.switchResult === "ok" ? modelName(child.target) : "")
      if (resolved) push("child_context", { model: resolved, agent: child.agent })
      push("child_done", { model: resolved, status, usage: child.usage })
      const batch = lines.join("\n") + "\n"
      batchAnswer(bin, "honor", batch)
      batchAnswer(bin, "end", batch)
    }

    // --- Routing: one call per subagent, made when its first prompt is
    // admitted. A session without a parent is the main session and is never
    // routed.
    await ctx.session.hook("prompt", async (event: any) => {
      try {
        if (closed) return
        const text = event?.prompt?.text
        const sessionID = typeof event?.sessionID === "string" ? event.sessionID : ""
        if (!sessionID || typeof text !== "string" || text.length === 0 || children.has(sessionID)) return
        const info: any = await ctx.session.get({ sessionID })
        const parentID = info && typeof info.parentID === "string" ? info.parentID : ""
        if (!parentID) return
        const requested = modelRef(info.model)
        if (!requested) return
        const child: Child = {
          sessionID,
          child: opaque(sessionID),
          parent: opaque(parentID),
          agent: typeof info.agent === "string" ? info.agent : "",
          directory:
            (info.location && typeof info.location.directory === "string" && info.location.directory) ||
            (typeof (ctx as any).location?.directory === "string" ? (ctx as any).location.directory : ""),
          requested,
          target: null,
          switchResult: "",
          contextModel: "",
          fallback: false,
          usage: { input: 0, output: 0, reasoning: 0, cache: { read: 0, write: 0 } },
          steps: new Set(),
          finished: false,
        }
        remember(child)
        const routed = answer(bin, [HARNESS], {
          ...base("route", "subagent_spawn_intent", child.child, child.parent),
          agent: child.agent,
          model: { providerID: requested.providerID, id: requested.id },
          // The pairs OpenCode lists right now, so the answer never names a
          // model the bridge cannot switch to.
          available: Array.from(await knownModels(ctx)),
          taskText: taskText(text),
          directory: child.directory,
        })
        if (!routed || routed.v !== WIRE_VERSION || routed.action !== "route") return
        const target = resolveTarget(routed.model, await knownModels(ctx))
        if (!target) return
        child.target = target
        try {
          await ctx.session.switchModel({ sessionID, model: { providerID: target.providerID, id: target.id } })
          child.switchResult = "ok"
        } catch {
          child.switchResult = "error"
        }
      } catch {
        // A bridge failure never blocks the child.
      }
    })

    // The agent loop reports the model it is about to call. The last report is
    // the child's real model, which is what the end record states.
    await ctx.session.hook("context", (event: any) => {
      try {
        const child = children.get(typeof event?.sessionID === "string" ? event.sessionID : "")
        if (!child) return
        const model = modelRef(event?.model)
        if (model) child.contextModel = modelName(model)
      } catch {
        // Observation only.
      }
    })

    // A routed child whose provider fails is returned to the model it asked
    // for, once. The undo is the only retry Downshift causes; every later
    // failure stays OpenCode's own policy.
    await ctx.session.hook("retry", async (event: any) => {
      try {
        const child = children.get(typeof event?.sessionID === "string" ? event.sessionID : "")
        if (!child || !child.target || child.switchResult !== "ok" || child.fallback) return
        const error = event?.error && typeof event.error === "object" ? event.error : {}
        const retried = answer(bin, [HARNESS], {
          ...base("retry", "child_retry", child.child, child.parent),
          agent: child.agent,
          model: { providerID: child.target.providerID, id: child.target.id },
          retry: {
            routed: true,
            requested: { providerID: child.requested.providerID, id: child.requested.id },
            errorType: typeof error.type === "string" ? error.type : "",
            status: count(error.status),
            attempt: count(event?.attempt),
          },
        })
        if (!retried || retried.v !== WIRE_VERSION || retried.action !== "retry") return
        const back = resolveTarget(retried.model, await knownModels(ctx))
        if (!back) return
        await ctx.session.switchModel({
          sessionID: child.sessionID,
          model: { providerID: back.providerID, id: back.id },
        })
        child.fallback = true
        event.decision = { retry: true, delay: 0 }
      } catch {
        // The harness's own retry policy stays in charge.
      }
    })

    // --- Shell compaction: the command is replaced in place, and only when
    // the shared shell policy says the wrapped form saves tokens.
    await ctx.tool.hook("execute.before", (event: any) => {
      try {
        if (event?.tool !== "shell") return
        const command = event?.input?.command
        if (typeof command !== "string" || command.length === 0) return
        const sessionID = typeof event?.sessionID === "string" ? event.sessionID : ""
        shellSessions.queue(command, sessionID)
      } catch {
        // Observation only.
      }
    })

    await ctx.shell.hook("create.before", (event: any) => {
      try {
        const command = typeof event?.command === "string" ? event.command : ""
        if (!command) return
        const sessionID = shellSessions.take(command)
        // An absent directory is sent empty: Go treats it as doubt and leaves
        // the command unchanged. Nothing here guesses one (process.cwd() is
        // not the session's directory).
        const located = (ctx as any).location?.directory
        const sessionDirectory = typeof located === "string" ? located : ""
        const directory = typeof event?.cwd === "string" ? event.cwd : sessionDirectory
        const hinted = answer(bin, [HARNESS], {
          ...base("shell", "shell_command", opaque(sessionID || `cwd\0${directory}`), ""),
          command,
          directory,
          // The shell can run elsewhere than the session; Go checks both.
          sessionDirectory,
        }, SHELL_TIMEOUT_MS)
        if (!hinted || hinted.v !== WIRE_VERSION || hinted.action !== "rewrite") return
        if (typeof hinted.command !== "string" || hinted.command.length === 0) return
        if (!acceptShellRewrite(command, hinted.command, bin)) return
        event.command = hinted.command
      } catch {
        // The command runs unchanged.
      }
    })

    // --- Child end and token usage. OpenCode reports the final token counts
    // of each step; the totals are summed once per assistant message.
    const onEvent = (event: any) => {
      try {
        const type = event?.type
        const data = event?.data
        const sessionID = data && typeof data.sessionID === "string" ? data.sessionID : ""
        if (!sessionID) return
        const child = children.get(sessionID)
        if (!child) return
        if (type === "session.step.ended") {
          const messageID = typeof data.assistantMessageID === "string" ? data.assistantMessageID : ""
          if (messageID && child.steps.has(messageID)) return
          if (messageID) child.steps.add(messageID)
          const tokens = data.tokens && typeof data.tokens === "object" ? data.tokens : {}
          const cache = tokens.cache && typeof tokens.cache === "object" ? tokens.cache : {}
          child.usage.input += count(tokens.input)
          child.usage.output += count(tokens.output)
          child.usage.reasoning += count(tokens.reasoning)
          child.usage.cache.read += count(cache.read)
          child.usage.cache.write += count(cache.write)
          return
        }
        if (type === "session.execution.succeeded") finish(child, "completed")
        else if (type === "session.execution.failed") finish(child, "failed")
      } catch {
        // The stream is an observation, never a gate.
      }
    }

    const controller = new AbortController()
    void (async () => {
      try {
        for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
          onEvent(event)
        }
      } catch {
        // A closed stream only stops observations.
      }
    })()

    return () => {
      closed = true
      try {
        controller.abort()
      } catch {
        // Already closed.
      }
    }
  },
})
