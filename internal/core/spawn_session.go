// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "github.com/tiagovilasboas/downshift/internal/quota"

// RecordSession stamps the session and quota evidence on the decision: whether
// the session list is known, the quota status of the model the spawn currently
// uses, and the evidence source. Every adapter did this by hand; it lives here
// so the five harnesses cannot drift apart.
func (d *Decision) RecordSession(session SessionList, currentModelID string) {
	d.SessionUnknown = !session.Known
	d.QuotaStatus = string(session.QuotaStatus(d.Harness, currentModelID))
	if session.Usage != nil {
		d.QuotaSource = quota.SourceName(session.Usage.Source)
	}
}

// PlanSession records the session evidence, builds the harness-agnostic rewrite
// plan for caps, and stamps the credit gate result and the quota status of the
// proposed model. Adapters then only apply their own wire-format rules to the
// returned plan.
func (d *Decision) PlanSession(caps HarnessCapabilities, res Resolver, session SessionList, currentModelID string) RewritePlan {
	d.RecordSession(session, currentModelID)
	plan := d.PlanForSession(caps, res, session)
	d.CreditHeld = plan.CreditHeld
	if plan.RewriteModel {
		d.QuotaStatus = string(session.QuotaStatus(d.Harness, plan.Model.ID))
	}
	return plan
}
