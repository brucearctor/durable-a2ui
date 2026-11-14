package incident

import (
	"fmt"

	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/contrib/workflowstreams"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

const TaskQueue = "incident-commander"

type IncidentInput struct {
	CommanderName    string                               `json:"commander_name"`
	ServiceName      string                               `json:"service_name"`
	IncidentSummary  string                               `json:"incident_summary"`
	StreamState      *workflowstreams.WorkflowStreamState `json:"stream_state,omitempty"`
	PendingMessages  []string                             `json:"pending_messages,omitempty"`
	TelemetryChecked bool                                 `json:"telemetry_checked,omitempty"`
	PolicyChecked    bool                                 `json:"policy_checked,omitempty"`
	Remediated       bool                                 `json:"remediated,omitempty"`
	MessageCounter   int                                  `json:"message_counter,omitempty"`
	TotalTokens      int                                  `json:"total_tokens,omitempty"`
	TotalCostUSD     float64                              `json:"total_cost_usd,omitempty"`
	TurnCount        int                                  `json:"turn_count,omitempty"`
	ToolCallCount    int                                  `json:"tool_call_count,omitempty"`
	SessionSnapshot  *googleadk.SessionSnapshot           `json:"session_snapshot,omitempty"`
	ApprovalRequired bool                                 `json:"approval_required,omitempty"`
	ApprovalDecision string                               `json:"approval_decision,omitempty"`
}

func (in *IncidentInput) ApplyDefaults() {
	if in.CommanderName == "" {
		in.CommanderName = "On-Call SRE"
	}
	if in.ServiceName == "" {
		in.ServiceName = "checkout-api (us-central1 & us-east1)"
	}
	if in.IncidentSummary == "" {
		in.IncidentSummary = "SEV-1: HTTP 5xx spike (18.4%) & AlloyDB connection pool saturation after canary v2.14.0-rc3 rollout"
	}
}

func BuildTelemetrySurface() []map[string]any {
	surfaceID := "telemetry-surface"
	comps := []map[string]any{
		{"id": "root", "component": "Card", "child": "tel_col"},
		{
			"id":        "tel_col",
			"component": "Column",
			"children":  []string{"tel_hdr", "tel_sub", "tel_m1", "tel_m2", "tel_m3", "tel_div", "tel_rc", "tel_btn"},
		},
		{"id": "tel_hdr", "component": "Text", "text": "📡 A2A Telemetry & Canary Diagnostics (:9003)", "variant": "h3"},
		{"id": "tel_sub", "component": "Text", "text": "Cluster: GKE us-central1 + us-east1 | Revision: checkout-api-canary-v2.14.0-rc3", "variant": "caption"},
		{"id": "tel_m1", "component": "Text", "text": "🔴 HTTP 5xx Rate: 18.4% (baseline 0.02%) — 14,200 req/s affected", "variant": "body"},
		{"id": "tel_m2", "component": "Text", "text": "🔴 p99 Latency: 4,820 ms | AlloyDB Connections: 200/200 (100% SATURATED)", "variant": "body"},
		{"id": "tel_m3", "component": "Text", "text": "🎯 Root-Cause Confidence: 96% (Unclosed pgx tx in payment retry middleware)", "variant": "h4"},
		{"id": "tel_div", "component": "Divider"},
		{"id": "tel_rc", "component": "Text", "text": "Recommended Mitigation: Roll back canary traffic to checkout-api-stable-v2.13.8 and drain leaked AlloyDB idle transactions.", "variant": "body"},
		{
			"id":        "tel_btn",
			"component": "Button",
			"child":     "tel_btn_lbl",
			"action":    a2ui.EventAction("Evaluate blast radius and SecOps policy", nil),
		},
		{"id": "tel_btn_lbl", "component": "Text", "text": "🛡️ Step 2: Evaluate Blast Radius & SecOps Policy (A2A) →"},
	}
	return []map[string]any{
		a2ui.CreateSurface(surfaceID, false),
		a2ui.UpdateComponents(surfaceID, comps),
	}
}

func BuildRunbookSurface() []map[string]any {
	surfaceID := "runbook-surface"
	comps := []map[string]any{
		{"id": "root", "component": "Card", "child": "rb_col"},
		{
			"id":        "rb_col",
			"component": "Column",
			"children":  []string{"rb_hdr", "rb_badge", "rb_freeze", "rb_burn", "rb_plan", "rb_btn"},
		},
		{"id": "rb_hdr", "component": "Text", "text": "🛡️ A2A SecOps Blast-Radius & Change-Freeze Policy (:9004)", "variant": "h3"},
		{"id": "rb_badge", "component": "Text", "text": "⚠️ POLICY STATUS: PASS_WITH_HITL_GATE (Tier-0 Checkout Service)", "variant": "caption"},
		{"id": "rb_freeze", "component": "Text", "text": "🔒 Active Change Freeze: Peak Traffic Launch Window", "variant": "body"},
		{"id": "rb_burn", "component": "Text", "text": "🔥 SLO Burn Rate: 18.4x (Monthly 99.95% error budget exhausts in 24 min)", "variant": "body"},
		{"id": "rb_plan", "component": "Text", "text": "📋 Approved Runbook RB-402: Shift 100% GKE Gateway weight to v2.13.8 + terminate idle AlloyDB transactions.", "variant": "h4"},
		{
			"id":        "rb_btn",
			"component": "Button",
			"child":     "rb_btn_lbl",
			"action":    a2ui.EventAction("Execute canary rollback RB-402 now!", nil),
		},
		{"id": "rb_btn_lbl", "component": "Text", "text": "🚀 Step 3: Execute Canary Rollback (RB-402) →"},
	}
	return []map[string]any{
		a2ui.CreateSurface(surfaceID, false),
		a2ui.UpdateComponents(surfaceID, comps),
	}
}

func BuildRemediationSurface(commander string) []map[string]any {
	surfaceID := "remediation-surface"
	comps := []map[string]any{
		{"id": "root", "component": "Card", "child": "rem_col"},
		{
			"id":        "rem_col",
			"component": "Column",
			"children":  []string{"rem_hdr", "rem_sub", "rem_m1", "rem_m2", "rem_m3"},
		},
		{"id": "rem_hdr", "component": "Text", "text": "✅ SEV-1 Resolved — Canary Rollback RB-402 Complete", "variant": "h3"},
		{"id": "rem_sub", "component": "Text", "text": fmt.Sprintf("Authorized by Incident Commander: %s | Revision: checkout-api-stable-v2.13.8", commander), "variant": "caption"},
		{"id": "rem_m1", "component": "Text", "text": "🟢 HTTP 5xx Error Rate: 18.4% → 0.01% (SLO Restored)", "variant": "body"},
		{"id": "rem_m2", "component": "Text", "text": "🟢 p99 Latency: 4,820 ms → 42 ms", "variant": "body"},
		{"id": "rem_m3", "component": "Text", "text": "🟢 AlloyDB Connection Pool: 200/200 → 38/200 active connections", "variant": "h4"},
	}
	return []map[string]any{
		a2ui.CreateSurface(surfaceID, false),
		a2ui.UpdateComponents(surfaceID, comps),
	}
}
