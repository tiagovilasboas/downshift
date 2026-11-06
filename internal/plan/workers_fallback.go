// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"strings"
)

// readOnlyPlaybooks are the playbooks whose work leaves files alone; any other
// playbook may edit and so needs a write worker.
var readOnlyPlaybooks = map[string]bool{"research": true, "review": true}

// editsFiles reports whether work under the playbook may write files.
func editsFiles(playbook string) bool { return !readOnlyPlaybooks[playbook] }

const turnOnHint = "Turn workers on with orchestration.workers in ~/.downshift/policy.json or DOWNSHIFT_WORKERS=read."

// workerWarning is empty when the worker state can run the part as a
// delegate step, else why it runs as a native spawn instead.
func workerWarning(p Part, w Workers) string {
	switch {
	case w.Mode != WorkersRead && w.Mode != WorkersWrite:
		return fmt.Sprintf("part %s: worker %q needs workers on and they are off; it runs as a native spawn. %s", p.ID, p.Worker, turnOnHint)
	case !w.Enabled(p.Worker):
		return fmt.Sprintf("part %s: worker %q is not enabled (enabled: %s); it runs as a native spawn. %s", p.ID, p.Worker, enabledList(w), turnOnHint)
	case editsFiles(p.Playbook) && !w.WriteEnabled():
		return fmt.Sprintf("part %s: playbook %s edits files, which needs write workers (mode is %s); it runs as a native spawn. Set orchestration.workers to write in ~/.downshift/policy.json or DOWNSHIFT_WORKERS=bypass.", p.ID, p.Playbook, w.Mode)
	}
	return ""
}

func enabledList(w Workers) string {
	if len(w.Agents) == 0 {
		return "none"
	}
	return strings.Join(w.Agents, ", ")
}
