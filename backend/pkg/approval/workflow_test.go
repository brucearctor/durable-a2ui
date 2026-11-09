package approval_test

import (
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"

	"github.com/ab/durable-a2ui/backend/pkg/approval"
)

func TestApprovalWorkflowDualSurfaceApprove(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(approval.ApprovalWorkflow)
	env.RegisterActivity(&approval.Activities{})

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("approve", "Approved for DevFest CDMX & Campus GPU Grant")
	}, 5*time.Second)

	env.ExecuteWorkflow(approval.ApprovalWorkflow, approval.ApprovalInput{
		RequestID:     "EXP-2026",
		Requester:     "Valeria Morales",
		Title:         "GDG DevFest CDMX & Campus Hackathon GPU Grant",
		Justification: "Live multi-agent workshop resources",
		Items: []approval.ExpenseItem{
			{Description: "GKE L4 GPU Cluster Quota", Amount: 485.00, Category: "Cloud Compute"},
		},
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("expected workflow to complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("unexpected workflow error: %v", err)
	}
	var res string
	if err := env.GetWorkflowResult(&res); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if res != "approved" {
		t.Fatalf("expected approved, got %s", res)
	}
}
