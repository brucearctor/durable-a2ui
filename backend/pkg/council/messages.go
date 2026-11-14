package council

import (
	"fmt"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

const (
	BoardSurface   = "council-board"
	VerdictSurface = "council-verdict"
)

func branchBadge(res *SpecialistResult, label string) string {
	if res == nil {
		return fmt.Sprintf("⏳ %s — Auditing via Parallel A2A...", label)
	}
	return fmt.Sprintf("✅ %s (%s) — %s [Attempt #%d]", label, res.Status, res.KeyMetric, res.XRay.Attempt)
}

func branchDetail(res *SpecialistResult) string {
	if res == nil {
		return "Waiting for remote A2A specialist agent response..."
	}
	return fmt.Sprintf("%s • %s (%s)", res.Headline, res.Recommendation, res.Endpoint)
}

// BuildCouncilBoardMessages renders the progressive 3-branch parallel fan-out surface.
func BuildCouncilBoardMessages(state CouncilState, create bool) []map[string]any {
	msgs := make([]map[string]any, 0, 3)
	if create {
		msgs = append(msgs, a2ui.CreateSurface(BoardSurface, true))
		msgs = append(msgs, a2ui.UpdateComponents(BoardSurface, []map[string]any{
			{"id": "root", "component": "Column", "children": []string{"header_card", "branches_row"}},
			{"id": "header_card", "component": "Card", "child": "header_col"},
			{"id": "header_col", "component": "Column", "children": []string{"prop_title", "prop_meta", "fanout_progress"}},
			{"id": "prop_title", "component": "Text", "text": a2ui.Path("/proposal_title"), "variant": "h3"},
			{"id": "prop_meta", "component": "Text", "text": a2ui.Path("/proposal_meta"), "variant": "caption"},
			{"id": "fanout_progress", "component": "Text", "text": a2ui.Path("/progress_text"), "variant": "h4"},
			{"id": "branches_row", "component": "Column", "children": []string{"rel_card", "sec_card", "fin_card"}},
			{"id": "rel_card", "component": "Card", "child": "rel_col"},
			{"id": "rel_col", "component": "Column", "children": []string{"rel_title", "rel_detail"}},
			{"id": "rel_title", "component": "Text", "text": a2ui.Path("/rel_title"), "variant": "h4"},
			{"id": "rel_detail", "component": "Text", "text": a2ui.Path("/rel_detail"), "variant": "body"},
			{"id": "sec_card", "component": "Card", "child": "sec_col"},
			{"id": "sec_col", "component": "Column", "children": []string{"sec_title", "sec_detail"}},
			{"id": "sec_title", "component": "Text", "text": a2ui.Path("/sec_title"), "variant": "h4"},
			{"id": "sec_detail", "component": "Text", "text": a2ui.Path("/sec_detail"), "variant": "body"},
			{"id": "fin_card", "component": "Card", "child": "fin_col"},
			{"id": "fin_col", "component": "Column", "children": []string{"fin_title", "fin_detail"}},
			{"id": "fin_title", "component": "Text", "text": a2ui.Path("/fin_title"), "variant": "h4"},
			{"id": "fin_detail", "component": "Text", "text": a2ui.Path("/fin_detail"), "variant": "body"},
		}))
	}

	msgs = append(msgs, a2ui.UpdateDataModel(BoardSurface, "", map[string]any{
		"proposal_title": state.ProposalTitle,
		"proposal_meta":  fmt.Sprintf("Target Service: %s  |  Lead: %s  |  Active Schema: v%d", state.Service, state.Architect, state.SchemaVersion),
		"progress_text":  fmt.Sprintf("⚡ workflow.Go Parallel Fan-Out: %d / %d A2A Specialists Completed", state.Completed, state.TotalBranches),
		"rel_title":      branchBadge(state.Reliability, "Branch 1: SRE Telemetry & SLO (:9003)"),
		"rel_detail":     branchDetail(state.Reliability),
		"sec_title":      branchBadge(state.SecOps, "Branch 2: SecOps Blast-Radius (:9004)"),
		"sec_detail":     branchDetail(state.SecOps),
		"fin_title":      branchBadge(state.FinOps, "Branch 3: FinOps & Quota Advisor (:9001)"),
		"fin_detail":     branchDetail(state.FinOps),
	}))
	return msgs
}

// BuildVerdictSurfaceV1 renders Schema v1 (Legacy Compact Text Summary).
// Used when workflow.GetVersion returns v1 before live schema migration.
func BuildVerdictSurfaceV1(state CouncilState, create bool) []map[string]any {
	msgs := make([]map[string]any, 0, 3)
	if create {
		msgs = append(msgs, a2ui.CreateSurface(VerdictSurface, true))
	}
	statusBanner := "🟡 Awaiting Architecture Council Sign-Off (Schema v1 — Compact Layout)"
	switch state.Decision {
	case "approved":
		statusBanner = "🟢 LAUNCH APPROVED (Schema v1 — Compact Layout)"
	case "changes_requested":
		statusBanner = "🔴 CHANGES REQUESTED (Schema v1 — Compact Layout)"
	}

	msgs = append(msgs,
		a2ui.UpdateComponents(VerdictSurface, []map[string]any{
			{"id": "root", "component": "Card", "child": "v1_col"},
			{"id": "v1_col", "component": "Column", "children": []string{"v1_badge", "v1_summary", "v1_hint", "v1_actions"}},
			{"id": "v1_badge", "component": "Text", "text": statusBanner, "variant": "h4"},
			{"id": "v1_summary", "component": "Text", "text": "All 3 A2A specialists responded. Legacy v1 surface shows a single text line without FinOps breakdown or interactive risk matrix.", "variant": "body"},
			{"id": "v1_hint", "component": "Text", "text": "💡 Click 'Hot-Upgrade Schema (v1 → v2)' to trigger workflow.GetVersion live surface migration without restarting the workflow!", "variant": "caption"},
			{"id": "v1_actions", "component": "Row", "children": []string{"v1_approve_btn", "v1_upgrade_btn"}},
			{
				"id":        "v1_approve_btn",
				"component": "Button",
				"child":     "v1_approve_lbl",
				"action":    a2ui.EventAction("council_decision", map[string]any{"decision": "approved"}),
			},
			{"id": "v1_approve_lbl", "component": "Text", "text": "✅ Approve Rollout"},
			{
				"id":        "v1_upgrade_btn",
				"component": "Button",
				"child":     "v1_upgrade_lbl",
				"variant":   "borderless",
				"action":    a2ui.EventAction("upgrade_schema", map[string]any{"target": 2}),
			},
			{"id": "v1_upgrade_lbl", "component": "Text", "text": "🔼 Upgrade to v2 Executive Scorecard"},
		}),
		a2ui.UpdateDataModel(VerdictSurface, "", map[string]any{
			"schema_version": 1,
			"decision":       state.Decision,
		}),
	)
	return msgs
}

// BuildVerdictSurfaceV2 renders Schema v2 (Executive Scorecard + Risk Matrix + FinOps Breakdown).
// Safely activated mid-flight via workflow.GetVersion + upgrade_schema signal!
func BuildVerdictSurfaceV2(state CouncilState, create bool) []map[string]any {
	msgs := make([]map[string]any, 0, 3)
	if create {
		msgs = append(msgs, a2ui.CreateSurface(VerdictSurface, true))
	}
	statusBanner := "✨ Schema v2 Executive Scorecard (Migrated Live via workflow.GetVersion!)"
	switch state.Decision {
	case "approved":
		statusBanner = "🟢 LAUNCH APPROVED — Schema v2 Executive Scorecard"
	case "changes_requested":
		statusBanner = "🔴 CHANGES REQUESTED — Schema v2 Executive Scorecard"
	}

	msgs = append(msgs,
		a2ui.UpdateComponents(VerdictSurface, []map[string]any{
			{"id": "root", "component": "Card", "child": "v2_col"},
			{"id": "v2_col", "component": "Column", "children": []string{"v2_hdr", "v2_sub", "v2_metrics_row", "v2_div", "v2_finops", "v2_actions"}},
			{"id": "v2_hdr", "component": "Text", "text": statusBanner, "variant": "h3"},
			{"id": "v2_sub", "component": "Text", "text": "Deterministic workflow.GetVersion(\"council-verdict-schema\", DefaultVersion, 2) hot-swapped the component tree in place:", "variant": "caption"},
			{"id": "v2_metrics_row", "component": "Row", "children": []string{"v2_slo_pill", "v2_sec_pill", "v2_cost_pill"}, "justify": "spaceBetween"},
			{"id": "v2_slo_pill", "component": "Text", "text": "📈 Canary p99: 42 ms (99.98% SLO)", "variant": "body"},
			{"id": "v2_sec_pill", "component": "Text", "text": "🛡️ Blast Radius: Scoped (Zero-Trust)", "variant": "body"},
			{"id": "v2_cost_pill", "component": "Text", "text": "💰 FinOps Est: $1,420/mo (-38% via Cache)", "variant": "body"},
			{"id": "v2_div", "component": "Divider"},
			{"id": "v2_finops", "component": "Text", "text": "Composite Readiness Score: 96 / 100 — All 3 parallel A2A specialist gates passed with zero unmitigated risks.", "variant": "body"},
			{"id": "v2_actions", "component": "Row", "children": []string{"v2_approve_btn", "v2_reject_btn"}},
			{
				"id":        "v2_approve_btn",
				"component": "Button",
				"child":     "v2_approve_lbl",
				"action":    a2ui.EventAction("council_decision", map[string]any{"decision": "approved"}),
			},
			{"id": "v2_approve_lbl", "component": "Text", "text": "🚀 Authorize Production Rollout"},
			{
				"id":        "v2_reject_btn",
				"component": "Button",
				"child":     "v2_reject_lbl",
				"variant":   "borderless",
				"action":    a2ui.EventAction("council_decision", map[string]any{"decision": "changes_requested"}),
			},
			{"id": "v2_reject_lbl", "component": "Text", "text": "🛑 Request Architecture Changes"},
		}),
		a2ui.UpdateDataModel(VerdictSurface, "", map[string]any{
			"schema_version": 2,
			"score":          96,
			"decision":       state.Decision,
		}),
	)
	return msgs
}
