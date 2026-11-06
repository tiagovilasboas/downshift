// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

// release is the explicit end-of-life for a worker: dropping its in-memory job
// record and deleting the on-disk session transcript it left behind, so a
// resumable session no longer lingers in the user's resume list once the caller
// is done with it (the isolation counterpart to session persistence, task T5 of
// the worker-continuation plan). The actual per-CLI deletion lives in the
// launcher (cleanup.go); this file is only the queue's side: which workers it
// releases, and the one place it asks the launcher to delete a session.

// Release lets go of settled workers the caller has no more work for: it drops
// each job's in-memory record and deletes its on-disk session transcript, so a
// resumable session no longer lingers once the caller is done with it. It is
// the explicit end-of-life for a worker that ended on its own (done, timeout or
// failed) and whose session send could otherwise have resumed.
//
// With no ids it releases every settled worker; with ids it releases only those,
// and refuses a worker that is still alive (queued, running or blocked) so a
// caller does not drop a worker mid-run by mistake: a live worker is stopped
// with cancel, not released. It returns the ids it released and, separately, a
// map of id to the clear reason each named id was refused (unknown id, or still
// running), so the tool can report both without an error that stops the batch.
// A transcript that could not be deleted is reported as a warning per id, not a
// failure: the job is still released.
func (q *Queue) Release(ids []string) (released []string, warnings map[string]string, refused map[string]string) {
	return q.ReleaseMarked(ids, MarkNone)
}

// ReleaseMarked is Release that also records the lead's verdict (used or
// discarded) on each released worker's report (report.go), which outlives the
// job. MarkNone records nothing.
func (q *Queue) ReleaseMarked(ids []string, mark Mark) (released []string, warnings map[string]string, refused map[string]string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	warnings = map[string]string{}
	refused = map[string]string{}
	targets := ids
	if len(ids) == 0 {
		// Release every settled worker, in start order for a stable result.
		for i := 1; i <= q.nextID; i++ {
			id := jobID(i)
			if j, ok := q.jobs[id]; ok && j.state.Finished() {
				targets = append(targets, id)
			}
		}
	}
	for _, id := range targets {
		j, ok := q.jobs[id]
		if !ok {
			refused[id] = "no worker has that id"
			continue
		}
		if !j.state.Finished() {
			refused[id] = "that worker is still running: stop it with cancel, do not release a live worker"
			continue
		}
		err := q.cleanupSessionLocked(j)
		if err != nil {
			warnings[id] = "the session transcript could not be deleted: " + err.Error()
		}
		q.reports.release(id, mark, err != nil)
		delete(q.jobs, id)
		released = append(released, id)
	}
	return released, warnings, refused
}

// Close ends the queue when the server exits, so no worker outlives the
// session that started it. It first releases every worker that already
// settled, deleting the session transcript each one left on disk (nothing can
// resume it once the server is gone), then stops every job that has not ended:
// a running worker is killed and a queued one never starts. A cancelled worker
// cleans up its own session on the cancel path, so it is not released twice.
func (q *Queue) Close() {
	q.Release(nil)
	q.mu.Lock()
	ids := make([]string, 0, len(q.jobs))
	for id, j := range q.jobs {
		if !j.state.Finished() {
			ids = append(ids, id)
		}
	}
	q.mu.Unlock()
	q.Cancel(ids)
}

// cleanupSessionLocked asks the launcher to delete the on-disk session this
// worker persisted, for every session the Downshift created (a captured id).
// It passes the effective directory the worker ran in, so a CLI that keys its
// transcript by the working directory (Claude) finds the right file: a write
// worker's worktree when it has one, else the spec's directory. A worker with
// no captured id persisted nothing, so there is nothing to delete. The caller
// holds the lock; the deletion itself touches only the user's CLI config dirs,
// never the queue. Returns the launcher's error so Release can warn on it.
func (q *Queue) cleanupSessionLocked(j *job) error {
	if j.sessionID == "" {
		return nil
	}
	cleaner, ok := q.launcher.(sessionCleaner)
	if !ok {
		return nil
	}
	spec := j.spec
	if j.worktree != "" {
		spec.Dir = j.worktree
	}
	return cleaner.CleanupSession(spec, j.sessionID)
}
