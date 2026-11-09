package incident_test

import (
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/testsuite"

	"github.com/ab/durable-a2ui/backend/pkg/incident"
)

func TestIncidentWorkflowWithA2AGuardrailAndCommanderHITL(t *testing.T) {
	t.Setenv("OFFLINE_MODE", "true")

	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(incident.IncidentWorkflow)

	acts, err := googleadk.NewActivities(googleadk.Config{
		Models: map[string]googleadk.ModelFactory{
			incident.ModelName: incident.NewResilientSREModelFactory(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create googleadk activities: %v", err)
	}
	env.RegisterActivityWithOptions(acts.InvokeModel, activity.RegisterOptions{Name: googleadk.InvokeModelActivityName})
	env.RegisterActivityWithOptions(incident.A2ADiagnoseTelemetryActivity, activity.RegisterOptions{Name: "a2a_diagnose_telemetry"})
	env.RegisterActivityWithOptions(incident.A2AEvaluateBlastRadiusActivity, activity.RegisterOptions{Name: "a2a_evaluate_blast_radius"})

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("user_message", "Investigate SEV-1 telemetry and canary diff")
	}, 1*time.Second)

	env.RegisterDelayedCallback(func() {
		// Jump straight to rollback -> triggers SecOps A2A policy guardrail -> triggers Commander HITL gate!
		env.SignalWorkflow("user_message", "Execute canary rollback RB-402 now!")
	}, 3*time.Second)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("commander_decision", "approved")
	}, 6*time.Second)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("end_session", nil)
	}, 10*time.Second)

	env.ExecuteWorkflow(incident.IncidentWorkflow, incident.IncidentInput{
		CommanderName: "CDMX SRE Lead",
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
	if res != "incident_closed" {
		t.Fatalf("expected incident_closed, got %s", res)
	}
}
