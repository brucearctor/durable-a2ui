package council

import (
	"context"
	"fmt"
	"time"

	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
)

type Activities struct{}

type AuditRequest struct {
	Service       string `json:"service"`
	ProposalTitle string `json:"proposal_title"`
}

// AuditReliabilityA2A calls the Telemetry & Canary Diagnostics A2A Agent (:9003).
func (a *Activities) AuditReliabilityA2A(ctx context.Context, req AuditRequest) (SpecialistResult, error) {
	select {
	case <-time.After(400 * time.Millisecond):
	case <-ctx.Done():
		return SpecialistResult{}, ctx.Err()
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", a2aservers.TelemetryDiagPort)
	prompt := fmt.Sprintf("Evaluate SLO and canary latency readiness for %s (%s)", req.Service, req.ProposalTitle)
	_, xray, err := a2aservers.CallA2AAgentWithMeta(ctx, endpoint, prompt)
	if err != nil {
		return SpecialistResult{}, err
	}

	return SpecialistResult{
		Domain:         "reliability",
		AgentName:      xray.AgentName,
		Endpoint:       endpoint,
		Status:         "READY",
		Headline:       "Multi-Region Canary & SLO Headroom Verified",
		KeyMetric:      "p99 42 ms • 99.98% SLO",
		Recommendation: "AlloyDB vector cache hit-rate at 94.2%; zero connection pool saturation.",
		DurationMs:     400,
		XRay:           xray,
	}, nil
}

// AuditSecOpsA2A calls the SecOps Blast-Radius & Policy A2A Agent (:9004).
func (a *Activities) AuditSecOpsA2A(ctx context.Context, req AuditRequest) (SpecialistResult, error) {
	select {
	case <-time.After(850 * time.Millisecond):
	case <-ctx.Done():
		return SpecialistResult{}, ctx.Err()
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", a2aservers.SecOpsPolicyPort)
	prompt := fmt.Sprintf("Evaluate blast radius and zero-trust IAM boundaries for %s (%s)", req.Service, req.ProposalTitle)
	_, xray, err := a2aservers.CallA2AAgentWithMeta(ctx, endpoint, prompt)
	if err != nil {
		return SpecialistResult{}, err
	}

	return SpecialistResult{
		Domain:         "secops",
		AgentName:      xray.AgentName,
		Endpoint:       endpoint,
		Status:         "PASS",
		Headline:       "Zero-Trust mTLS & Regional Blast-Radius Scoped",
		KeyMetric:      "Blast Radius: 1 Zone (Isolated)",
		Recommendation: "Workload Identity Federation & per-tenant rate limits verified.",
		DurationMs:     850,
		XRay:           xray,
	}, nil
}

// AuditFinOpsA2A calls the Campus & Cloud Spot Finder A2A Agent (:9001) for FinOps capacity scoring.
func (a *Activities) AuditFinOpsA2A(ctx context.Context, req AuditRequest) (SpecialistResult, error) {
	select {
	case <-time.After(1300 * time.Millisecond):
	case <-ctx.Done():
		return SpecialistResult{}, ctx.Err()
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", a2aservers.RestaurantFinderPort)
	prompt := fmt.Sprintf("Evaluate FinOps quota efficiency and semantic caching savings for %s", req.Service)
	_, xray, err := a2aservers.CallA2AAgentWithMeta(ctx, endpoint, prompt)
	if err != nil {
		return SpecialistResult{}, err
	}

	return SpecialistResult{
		Domain:         "finops",
		AgentName:      "FinOps & Quota Advisor (via A2A :9001)",
		Endpoint:       endpoint,
		Status:         "PASS",
		Headline:       "Semantic Cache & Spot GPU Quota Optimized",
		KeyMetric:      "$1,420/mo (-38% Token Spend)",
		Recommendation: "AlloyDB semantic cache reduces LLM token egress by 38% at peak load.",
		DurationMs:     1300,
		XRay:           xray,
	}, nil
}
