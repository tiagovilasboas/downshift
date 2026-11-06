// Downshift OpenCode bridge module (wire v1). Written by `downshift setup`; edit nothing here.
//
// Shell session tagging: tool.execute.before carries the session id and runs
// before the shell command exists; shell.create.before carries the command and
// runs after. This module joins the two by command text.

// A pending session tag older than this never reached shell creation (the call
// was denied or failed first) and is pruned.
const PENDING_SHELL_TTL_MS = 30000
// At most this many pending tags are kept across all commands; the oldest go
// first.
const MAX_PENDING_TAGS = 500

export function createShellSessions() {
  // Entries are never dropped by session id: one session can run several shell
  // calls in parallel. A call denied or failing before creation leaves its
  // entry behind, so an entry older than the TTL is pruned whenever the map is
  // touched. Residual case: another session's identical command within the TTL
  // of a denied call can take its tag (telemetry only).
  type Pending = { sessionID: string; at: number; seq: number }
  let seq = 0
  const pending = new Map<string, Pending[]>()
  const prune = () => {
    const now = Date.now()
    for (const [command, queue] of pending) {
      const kept = queue.filter((p) => now - p.at < PENDING_SHELL_TTL_MS)
      if (kept.length === 0) pending.delete(command)
      else if (kept.length !== queue.length) pending.set(command, kept)
    }
  }
  // dropOldestBeyondCap keeps the total number of pending tags, over every
  // command, at MAX_PENDING_TAGS: a command repeated within the TTL cannot
  // grow the map without bound.
  const dropOldestBeyondCap = () => {
    let total = 0
    for (const queue of pending.values()) total += queue.length
    for (; total > MAX_PENDING_TAGS; total--) {
      let oldestCommand = ""
      let oldestSeq = Infinity
      for (const [command, queue] of pending) {
        if (queue[0].seq < oldestSeq) {
          oldestSeq = queue[0].seq
          oldestCommand = command
        }
      }
      const queue = pending.get(oldestCommand)!
      queue.shift()
      if (queue.length === 0) pending.delete(oldestCommand)
    }
  }
  return {
    // queue records the session about to run `command`.
    queue(command: string, sessionID: string): void {
      prune()
      const queue = pending.get(command) || []
      queue.push({ sessionID, at: Date.now(), seq: seq++ })
      pending.set(command, queue)
      dropOldestBeyondCap()
    },
    // take returns the oldest live session for `command`, or "".
    take(command: string): string {
      prune()
      const queue = pending.get(command)
      const sessionID = queue && queue.length > 0 ? queue.shift()!.sessionID : ""
      if (queue && queue.length === 0) pending.delete(command)
      return sessionID
    },
  }
}
