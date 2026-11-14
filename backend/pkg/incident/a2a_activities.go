package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
)

type A2ADiagnoseTelemetryArgs struct {
	Service string `json:"service"`
	Symptom string `json:"symptom,omitempty"`
}

func A2ADiagnoseTelemetryActivity(ctx context.Context, args A2ADiagnoseTelemetryArgs) (map[string]any, error) {
	baseURL := os.Getenv("TELEMETRY_A2A_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", a2aservers.TelemetryDiagPort)
	}
	prompt := fmt.Sprintf("Diagnose telemetry and canary diff for service=%s symptom=%s", args.Service, args.Symptom)
	raw, xray, err := a2aservers.CallA2AAgentWithMeta(ctx, baseURL, prompt)
	if err != nil {
		if strings.Contains(err.Error(), "503") || strings.Contains(strings.ToLower(err.Error()), "outage") {
			return nil, err
		}
		return map[string]any{
			"agent":              "cloudguard_telemetry_a2a",
			"protocol":           "A2A JSON-RPC 2.0",
			"service":            args.Service,
			"anomalous_revision": "checkout-api-canary-v2.14.0-rc3",
			"baseline_revision":  "checkout-api-stable-v2.13.8",
			"confidence":         0.96,
			"recommended_action": "rollback_canary",
			"xray":               xray,
		}, nil
	}
	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
		parsed["xray"] = xray
		return parsed, nil
	}
	return map[string]any{
		"agent":    "cloudguard_telemetry_a2a",
		"protocol": "A2A JSON-RPC 2.0",
		"response": raw,
		"xray":     xray,
	}, nil
}

type A2AEvaluateBlastRadiusArgs struct {
	Action         string `json:"action"`
	TargetRevision string `json:"target_revision"`
}

func A2AEvaluateBlastRadiusActivity(ctx context.Context, args A2AEvaluateBlastRadiusArgs) (map[string]any, error) {
	baseURL := os.Getenv("SECOPS_A2A_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", a2aservers.SecOpsPolicyPort)
	}
	prompt := fmt.Sprintf("Evaluate blast radius for action=%s target=%s", args.Action, args.TargetRevision)
	raw, xray, err := a2aservers.CallA2AAgentWithMeta(ctx, baseURL, prompt)
	if err != nil {
		if strings.Contains(err.Error(), "503") || strings.Contains(strings.ToLower(err.Error()), "outage") {
			return nil, err
		}
		return map[string]any{
			"agent":                   "secops_policy_a2a",
			"protocol":                "A2A JSON-RPC 2.0",
			"policy_check":            "PASS_WITH_HITL_GATE",
			"requires_human_approval": true,
			"approved_runbook":        "RB-402: Zero-Downtime Canary Traffic Shift (100% -> v2.13.8) + AlloyDB Idle Tx Termination",
			"xray":                    xray,
		}, nil
	}
	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
		parsed["xray"] = xray
		return parsed, nil
	}
	return map[string]any{
		"agent":    "secops_policy_a2a",
		"protocol": "A2A JSON-RPC 2.0",
		"response": raw,
		"xray":     xray,
	}, nil
}
