package order

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/contrib/workflowstreams"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var orderStages = []struct {
	name  string
	delay time.Duration
}{
	{name: "confirmed", delay: 8 * time.Second},
	{name: "preparing", delay: 10 * time.Second},
	{name: "out_for_delivery", delay: 10 * time.Second},
	{name: "delivered", delay: 0},
}

func OrderWorkflow(ctx workflow.Context, input OrderInput) (string, error) {
	stream, err := workflowstreams.NewWorkflowStream(ctx, input.StreamState)
	if err != nil {
		return "", err
	}
	ui := stream.Topic("a2ui")

	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	shortID := strings.ToUpper(wfID)
	if idx := strings.LastIndex(shortID, "-"); idx != -1 && idx+1 < len(shortID) {
		shortID = shortID[idx+1:]
	}

	var total float64
	for _, item := range input.Items {
		total += item.Price * float64(item.Qty)
	}

	state := OrderState{
		OrderID:    shortID,
		Customer:   input.Customer,
		Address:    input.Address,
		Items:      input.Items,
		Total:      total,
		Status:     "confirmed",
		Step:       1,
		TotalSteps: len(orderStages),
		ETAMinutes: 35,
	}

	rushCh := workflow.GetSignalChannel(ctx, "rush_order")
	ffCh := workflow.GetSignalChannel(ctx, "fast_forward")
	cancelCh := workflow.GetSignalChannel(ctx, "cancel_order")

	fastForwardPending := false
	drainSignals := func() {
		for {
			var v any
			if rushCh.ReceiveAsync(&v) {
				if !state.Rushed {
					state.Rushed = true
					if state.ETAMinutes > 1 {
						state.ETAMinutes /= 2
					}
					for _, m := range UpdateStepProgress(state) {
						ui.Publish(m)
					}
				}
				continue
			}
			if ffCh.ReceiveAsync(&v) {
				fastForwardPending = true
				continue
			}
			if cancelCh.ReceiveAsync(&v) {
				if !state.Cancelled && state.Status != "delivered" {
					state.Cancelled = true
					state.Status = "cancelled"
					state.CancelReason = fmt.Sprintf("Cancelled by %s", state.Customer)
					for _, m := range UpdateCancelled(state) {
						ui.Publish(m)
					}
				}
				continue
			}
			break
		}
	}

	for _, m := range CreateOrderSurface() {
		ui.Publish(m)
	}
	for _, m := range BuildInitialComponents(state) {
		ui.Publish(m)
	}
	for _, m := range BuildInitialDataModel(state) {
		ui.Publish(m)
	}

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	})
	var acts *Activities

	for idx, stg := range orderStages {
		drainSignals()
		if state.Cancelled {
			return "cancelled", nil
		}

		state.Step = idx + 1
		state.Status = stg.name

		var res StageResult
		err := workflow.ExecuteActivity(actCtx, acts.ExecuteStage, StageRequest{
			OrderID: state.OrderID,
			Stage:   stg.name,
			Rushed:  state.Rushed,
		}).Get(ctx, &res)
		if err != nil {
			return "", err
		}

		state.ETAMinutes = res.ETAMinutes
		if res.Driver != nil && state.Driver == nil {
			state.Driver = res.Driver
			for _, m := range AddDriverCard(state) {
				ui.Publish(m)
			}
		}

		for _, m := range UpdateStepProgress(state) {
			ui.Publish(m)
		}

		if stg.delay > 0 {
			waitDur := stg.delay
			if state.Rushed {
				waitDur = waitDur / 2
			}
			timerCtx, cancelTimer := workflow.WithCancel(ctx)
			timerFuture := workflow.NewTimer(timerCtx, waitDur)
			timerFired := false

			for !timerFired && !fastForwardPending && !state.Cancelled {
				sel := workflow.NewSelector(ctx)
				sel.AddFuture(timerFuture, func(f workflow.Future) {
					timerFired = true
				})
				sel.AddReceive(rushCh, func(c workflow.ReceiveChannel, more bool) {
					var v any
					c.Receive(ctx, &v)
					if !state.Rushed {
						state.Rushed = true
						if state.ETAMinutes > 1 {
							state.ETAMinutes /= 2
						}
						for _, m := range UpdateStepProgress(state) {
							ui.Publish(m)
						}
					}
				})
				sel.AddReceive(ffCh, func(c workflow.ReceiveChannel, more bool) {
					var v any
					c.Receive(ctx, &v)
					fastForwardPending = true
				})
				sel.AddReceive(cancelCh, func(c workflow.ReceiveChannel, more bool) {
					var v any
					c.Receive(ctx, &v)
					if !state.Cancelled {
						state.Cancelled = true
						state.Status = "cancelled"
						state.CancelReason = fmt.Sprintf("Cancelled by %s", state.Customer)
						for _, m := range UpdateCancelled(state) {
							ui.Publish(m)
						}
					}
				})
				sel.Select(ctx)
			}
			cancelTimer()
			fastForwardPending = false

			if state.Cancelled {
				return "cancelled", nil
			}
		}
	}

	return "delivered", nil
}
