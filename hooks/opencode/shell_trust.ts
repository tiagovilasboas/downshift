// Downshift OpenCode bridge module (wire v1). Written by `downshift setup`; edit nothing here.
//
// The shell trust boundary: which binary the bridge runs, and which rewrite of
// a shell command it accepts.

import { realpathSync, statSync } from "node:fs"
import { delimiter, isAbsolute, join } from "node:path"

// resolveBin picks the binary the bridge runs, as an absolute path, once at
// setup. DOWNSHIFT_BIN, when set, must be an absolute path to an existing
// regular file. Unset, `downshift` is looked up on PATH now and never again, so
// a later PATH change cannot swap the binary. Anything unresolved returns null
// and the bridge does nothing (every hook passes through, as if Downshift were
// not installed).
export function resolveBin(env: string | undefined, path: string | undefined = process.env.PATH): string | null {
  if (env === undefined || env === "") return findOnPath("downshift", path)
  try {
    if (isAbsolute(env) && statSync(env).isFile()) return env
  } catch {
    // Missing or unreadable: rejected below.
  }
  return null
}

// findOnPath returns the first regular file called `name` in an absolute PATH
// entry. Relative entries are skipped: they would depend on the working folder.
function findOnPath(name: string, path: string | undefined): string | null {
  const names = process.platform === "win32" ? [name + ".exe", name] : [name]
  for (const dir of (path || "").split(delimiter)) {
    if (!isAbsolute(dir)) continue
    for (const n of names) {
      try {
        const candidate = join(dir, n)
        const st = statSync(candidate)
        // Without an execute bit the file cannot run (POSIX); skip it so a
        // later PATH entry can be the one found.
        if (st.isFile() && (process.platform === "win32" || (st.mode & 0o111) !== 0)) return candidate
      } catch {
        // Not here; keep looking.
      }
    }
  }
  return null
}

function sameFile(a: string, b: string): boolean {
  try {
    return realpathSync(a) === realpathSync(b)
  } catch {
    return a === b
  }
}

// REWRAP matches the prefix Downshift puts before a command it wraps: the
// absolute, single-quoted binary path, ` compact `, an optional hashed
// session tag, and `-- ` (internal/execwrap/policy.go: QuoteExecutable,
// execTail, WithSession).
const REWRAP = /'(\/[^'\u0000-\u001f\u007f]*)' compact(?: --session=[0-9a-f]+)? -- /g

// UNQUOTED is a command with no quoting, expansion or control character: no
// quote of either kind, no `$`, no backtick, no backslash, and nothing in the control range
// (a tab or newline included). Only such a command is rewritten, so the
// wrapper cannot land inside a quoted or expanded region.
const UNQUOTED = /^[^'"$`\\\u0000-\u001f\u007f]*$/

// SEPARATOR ends a command: `;`, `&&`, `||` or `|`. A `|` that follows `>`
// is the noclobber redirect `>|`, not a separator.
const SEPARATOR = /&&|\|\||;|\|/g

// ASSIGNMENT is an environment word that may precede a command.
const ASSIGNMENT = /^[A-Za-z_][A-Za-z0-9_]*=\S*$/

// atCommandStart reports whether `before`, the text ahead of a wrapper, leaves
// the wrapper in command position: after the last separator (or the start)
// there is nothing but NAME=value words. A wrapper after an option, a
// redirection or any other word is an argument of another command, not a
// command being wrapped.
function atCommandStart(before: string): boolean {
  let end = 0
  for (const m of before.matchAll(SEPARATOR)) {
    if (m[0] === "|" && before[m.index! - 1] === ">") continue
    end = m.index! + m[0].length
  }
  return before
    .slice(end)
    .split(" ")
    .filter((w) => w !== "")
    .every((w) => ASSIGNMENT.test(w))
}

// acceptShellRewrite reports whether `rewritten` is exactly the shape Downshift
// emits for `original`: the original command with one or more wrapper prefixes
// added (a compound or chain wraps each allowlisted segment) and nothing else.
// A wrapper must be in command position (see atCommandStart) and name the
// pinned binary. Runs of ASCII spaces compare as one, because the Go side
// joins the words of a wrapped segment with single spaces; any other change
// between words differs, so a rewrite cannot split or merge arguments.
// Anything else leaves the command unchanged.
export function acceptShellRewrite(original: string, rewritten: string, pinned: string): boolean {
  try {
    if (!UNQUOTED.test(original)) return false
    const wrappers: string[] = []
    let misplaced = false
    const stripped = rewritten.replace(REWRAP, (_m, path: string, offset: number) => {
      if (!atCommandStart(rewritten.slice(0, offset))) misplaced = true
      wrappers.push(path)
      return ""
    })
    if (wrappers.length === 0 || misplaced) return false
    const words = (t: string) => t.replace(/ +/g, " ").replace(/^ | $/g, "")
    if (words(stripped) !== words(original)) return false
    return wrappers.every((w) => statSync(w).isFile() && sameFile(w, pinned))
  } catch {
    return false
  }
}
