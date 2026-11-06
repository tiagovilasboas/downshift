// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

// release lets go of workers that have finished and deletes their on-disk
// session transcripts. It is the MCP surface over Queue.Release (worker
// continuation, task T5): the queue owns the end-of-life and the safe deletion,
// this file only maps the arguments in and reports what happened. It is
// registered by WorkTools in work_tools.go; kept here so each worker tool stays
// a readable file.

const releaseSchema = `{"type":"object","properties":{"job_ids":{"type":"array","minItems":1,"maxItems":16,"items":{"type":"string","maxLength":16},"description":"The ids of the finished workers to release. Omit it to release every worker that has finished."},"mark":{"type":"string","enum":["used","discarded"],"description":"Your verdict on the result of the workers released by this call. status lists it in each worker's report."}},"additionalProperties":false}`

const releaseDesc = "Lets go of workers that have finished, so their resumable sessions no longer linger: it drops each worker from the queue and deletes the on-disk session transcript the worker wrote, for every session Downshift created. Pass job_ids to release named workers, or omit it to release every finished worker at once. A worker that is still running is refused (stop it with cancel first, do not release a live worker), and an unknown id is refused; each refusal is reported per id without stopping the rest. Release a worker once you have collected its result and have no follow-up for it; use send instead to give it more work on the same conversation. A transcript that could not be deleted is reported as a warning, not a failure: the worker is still released. Pass mark (used or discarded) to record whether you used the results of the workers this call releases; the status tool keeps that verdict in each worker's report, also after the release."

type releaseArgs struct {
	JobIDs []string `json:"job_ids"`
	Mark   string   `json:"mark"`
}

// release drops settled workers and deletes their session transcripts, through
// Queue.Release: with ids it releases those finished workers, with none it
// releases every finished worker. The queue owns the deletion (exact file, in
// the CLI's own config dir, id validated, no symlink followed, no wide glob);
// this tool only maps the arguments in and reports the three outcomes the queue
// separates: released ids, per-id warnings for a transcript that could not be
// deleted, and per-id refusals for an unknown or still-running id.
func (d Deps) release(ctx context.Context, raw json.RawMessage) (Result, error) {
	if d.Work == nil {
		return Result{}, errNoWork
	}
	var a releaseArgs
	if err := decodeStrict(raw, &a); err != nil {
		return Result{}, err
	}
	mark, ok := workers.ParseMark(a.Mark)
	if !ok {
		return Result{}, errors.New("mark must be used or discarded")
	}
	released, warnings, refused := d.Work.ReleaseMarked(a.JobIDs, mark)
	data := map[string]any{"released": orEmpty(released)}
	if len(warnings) > 0 {
		data["warnings"] = warnings
	}
	if len(refused) > 0 {
		data["refused"] = refused
	}
	return d.envelope(ctx, outcome{data: data, harnesses: workers.WorkerHarnesses()})
}

// orEmpty returns a non-nil slice so the released field is always an array in
// the reply, even when nothing was released (every id was refused), rather than
// JSON null.
func orEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
