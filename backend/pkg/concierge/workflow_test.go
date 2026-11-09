package concierge_test

import (
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/testsuite"

	"github.com/ab/durable-a2ui/backend/pkg/concierge"
	"github.com/ab/durable-a2ui/backend/pkg/order"
)

func TestConciergeWorkflowFullPipelineWithGuardrailAndChildWorkflow(t *testing.T) {
	t.Setenv("OFFLINE_MODE", "true")

	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(concierge.ConciergeWorkflow)
	env.RegisterWorkflow(order.OrderWorkflow)
	env.RegisterActivity(&order.Activities{})

	acts, err := googleadk.NewActivities(googleadk.Config{
		Models: map[string]googleadk.ModelFactory{
			concierge.ModelName: concierge.NewResilientModelFactory(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create googleadk activities: %v", err)
	}
	env.RegisterActivityWithOptions(acts.InvokeModel, activity.RegisterOptions{Name: googleadk.InvokeModelActivityName})
	env.RegisterActivityWithOptions(concierge.A2AFindRestaurantsActivity, activity.RegisterOptions{Name: "a2a_find_restaurants"})
	env.RegisterActivityWithOptions(concierge.A2ACheckDietarySafetyActivity, activity.RegisterOptions{Name: "a2a_check_dietary_safety"})

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("user_message", "What restaurants do you recommend?")
	}, 500*time.Millisecond)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("user_message", "Show me the menu")
	}, 1*time.Second)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("user_message", "Order 2 Tacos al Pastor")
	}, 3*time.Second)

	env.RegisterDelayedCallback(func() {
		// Triggers programmatic dietary safety guardrail -> A2A dietary check -> confirm_order -> OrderWorkflow child!
		env.SignalWorkflow("user_message", "Confirm my order!")
	}, 5*time.Second)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("end_session", nil)
	}, 10*time.Second)

	env.ExecuteWorkflow(concierge.ConciergeWorkflow, concierge.ConciergeInput{
		CustomerName: "DevFest Attendee",
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
	if res != "session_ended" {
		t.Fatalf("expected session_ended, got %s", res)
	}
}
