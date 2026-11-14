package council_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"

	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
	"github.com/ab/durable-a2ui/backend/pkg/council"
)

func TestCouncilWorkflowParallelFanOutAndLiveSchemaMigration(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(council.CouncilWorkflow)

	var acts *council.Activities
	env.OnActivity(acts.AuditReliabilityA2A, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ council.AuditRequest) (council.SpecialistResult, error) {
			return council.SpecialistResult{
				Domain:    "reliability",
				AgentName: "CloudGuard Telemetry A2A",
				Endpoint:  "http://127.0.0.1:9003",
				Status:    "READY",
				Headline:  "SLO Verified",
				KeyMetric: "p99 42 ms",
				XRay:      a2aservers.A2ACallMeta{Attempt: 1},
			}, nil
		},
	)
	env.OnActivity(acts.AuditSecOpsA2A, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ council.AuditRequest) (council.SpecialistResult, error) {
			return council.SpecialistResult{
				Domain:    "secops",
				AgentName: "SecOps Blast-Radius A2A",
				Endpoint:  "http://127.0.0.1:9004",
				Status:    "PASS",
				Headline:  "Zero-Trust Scoped",
				KeyMetric: "1 Zone",
				XRay:      a2aservers.A2ACallMeta{Attempt: 1},
			}, nil
		},
	)
	env.OnActivity(acts.AuditFinOpsA2A, mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ council.AuditRequest) (council.SpecialistResult, error) {
			return council.SpecialistResult{
				Domain:    "finops",
				AgentName: "FinOps Advisor A2A",
				Endpoint:  "http://127.0.0.1:9001",
				Status:    "PASS",
				Headline:  "Cache Savings",
				KeyMetric: "$1,420/mo",
				XRay:      a2aservers.A2ACallMeta{Attempt: 1},
			}, nil
		},
	)

	// Hot-upgrade schema from v1 -> v2 at 2s, then approve rollout at 4s
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("upgrade_schema", 2)
	}, 2*time.Second)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("council_decision", "approved")
	}, 4*time.Second)

	env.ExecuteWorkflow(council.CouncilWorkflow, council.CouncilInput{
		ProposalTitle: "RFC-804: Multi-Region LLM Gateway",
		TargetSchema:  1,
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("expected CouncilWorkflow to complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decision string
	if err := env.GetWorkflowResult(&decision); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if decision != "approved" {
		t.Fatalf("expected approved, got %s", decision)
	}
}
