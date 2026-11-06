// Downshift OpenCode bridge module (wire v1). Written by `downshift setup`; edit nothing here.
//
// Model references: reading OpenCode's model shape and matching an answered
// model against the pairs OpenCode lists.

export type ModelRef = { providerID: string; id: string }

// modelRef reads OpenCode's model shape. A model reported as a
// "provider/id" string is accepted too, because the session inventory can
// come back in either form.
export function modelRef(value: any): ModelRef | null {
  if (!value || typeof value !== "object") return null
  const providerID = typeof value.providerID === "string" ? value.providerID : ""
  const id = typeof value.id === "string" ? value.id : ""
  if (!providerID || !id) return null
  return { providerID, id }
}

// resolveTarget turns an answered model into the pair switchModel takes, and
// only when OpenCode lists that pair. An allowlist entry from `opencode models`
// is "provider/id", so the answer may name the pair whole or name the harness
// provider with the qualified id. switchModel accepts a pair it does not know
// and the child then fails with "Model unavailable", which no retry covers, so
// a pair OpenCode does not list keeps the child on its own model. `known` is
// the set of "provider/id" pairs OpenCode lists right now.
export function resolveTarget(value: any, known: Set<string>): ModelRef | null {
  const model = modelRef(value)
  if (!model) return null
  if (known.has(`${model.providerID}/${model.id}`)) return model
  const slash = model.id.indexOf("/")
  if (slash > 0) {
    const split = { providerID: model.id.slice(0, slash), id: model.id.slice(slash + 1) }
    if (known.has(`${split.providerID}/${split.id}`)) return split
  }
  return null
}

// knownModels is the set of "provider/id" pairs OpenCode lists right now. A
// listing that fails returns an empty set, and nothing is routed.
export async function knownModels(ctx: any): Promise<Set<string>> {
  const known = new Set<string>()
  try {
    const listed: any = await ctx.model?.list?.()
    const rows: any[] = Array.isArray(listed) ? listed : Array.isArray(listed?.data) ? listed.data : []
    for (const row of rows) {
      const id = typeof row?.id === "string" ? row.id : typeof row?.modelID === "string" ? row.modelID : ""
      if (typeof row?.providerID === "string" && id) known.add(`${row.providerID}/${id}`)
    }
  } catch {
    // Nothing is known, so nothing is routed.
  }
  return known
}

export function modelName(model: ModelRef | null): string {
  return model ? `${model.providerID}/${model.id}` : ""
}
