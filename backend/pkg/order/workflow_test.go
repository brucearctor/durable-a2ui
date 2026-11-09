package order_test

import (
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"

	"github.com/ab/durable-a2ui/backend/pkg/order"
)

func TestOrderWorkflowCompletes(t *testing.T) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(order.OrderWorkflow)
	env.RegisterActivity(&order.Activities{})

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("rush_order", nil)
	}, 2*time.Second)

	env.ExecuteWorkflow(order.OrderWorkflow, order.OrderInput{
		Customer: "Test Student",
		Address:  "Campus CS Building",
		Items: []order.OrderItem{
			{Name: "Tacos al Pastor", Qty: 2, Price: 11.50},
		},
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("expected workflow to complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("unexpected workflow error: %v", err)
	}
	var status string
	if err := env.GetWorkflowResult(&status); err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if status != "delivered" {
		t.Fatalf("expected status delivered, got %s", status)
	}
}
