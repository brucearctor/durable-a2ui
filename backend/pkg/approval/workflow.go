package approval

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

func ApprovalWorkflow(ctx workflow.Context, input ApprovalInput) (string, error) {
	stream, err := a2ui.NewStream(ctx, input.StreamState)
	if err != nil {
		return "", err
	}
	requesterUI := stream.Topic("requester-ui")
	approverUI := stream.Topic("approver-ui")

	var total float64
	for _, item := range input.Items {
		total += item.Amount
	}

	reqApprovals := input.RequiredApprovals
	if reqApprovals <= 0 {
		reqApprovals = 1
	}

	state := ApprovalState{
		RequestID:         input.RequestID,
		Requester:         input.Requester,
		Title:             input.Title,
		Justification:     input.Justification,
		Items:             input.Items,
		Total:             total,
		Status:            "submitted",
		RequiredApprovals: reqApprovals,
	}

	var decision string
	var decisionNote string

	publishRequester := func(msgs []map[string]any) {
		for _, m := range msgs {
			_ = requesterUI.Publish(m)
		}
	}
	publishApprover := func(msgs []map[string]any) {
		for _, m := range msgs {
			_ = approverUI.Publish(m)
		}
	}

	recordApprove := func(note string) {
		if decision != "" {
			return
		}
		state.ApprovalCount++
		if state.ApprovalCount >= state.RequiredApprovals {
			decision = "approved"
			if note == "" {
				decisionNote = "Approved by manager"
			} else {
				decisionNote = note
			}
		} else {
			publishRequester(UpdateRequesterStatus(state))
			publishApprover(UpdateApproverQuorumProgress(state))
		}
	}

	recordReject := func(note string) {
		if decision != "" {
			return
		}
		state.RejectionCount++
		if state.RejectionCount >= state.RequiredApprovals {
			decision = "rejected"
			if note == "" {
				decisionNote = "Rejected by manager"
			} else {
				decisionNote = note
			}
		} else {
			publishRequester(UpdateRequesterStatus(state))
			publishApprover(UpdateApproverQuorumProgress(state))
		}
	}

	approveCh := workflow.GetSignalChannel(ctx, "approve")
	rejectCh := workflow.GetSignalChannel(ctx, "reject")
	drainSignals := func() {
		for {
			var note string
			if approveCh.ReceiveAsync(&note) {
				recordApprove(note)
				continue
			}
			if rejectCh.ReceiveAsync(&note) {
				recordReject(note)
				continue
			}
			break
		}
	}

	// Step 1: Create both surfaces and initial views
	publishRequester(CreateRequesterSurface())
	publishRequester(BuildRequesterComponents(state))
	publishRequester(BuildRequesterData(state))

	publishApprover(CreateApproverSurface())
	publishApprover(BuildApproverComponents(state))
	publishApprover(BuildApproverData(state))

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

	// Step 2: Validate expense & notify manager
	_ = workflow.ExecuteActivity(actCtx, acts.ValidateExpense, ValidateReq{
		RequestID: state.RequestID,
		Total:     state.Total,
	}).Get(ctx, nil)

	state.Status = "under_review"
	publishRequester(UpdateRequesterStatus(state))

	_ = workflow.ExecuteActivity(actCtx, acts.NotifyManager, NotifyManagerReq{
		RequestID: state.RequestID,
		Requester: state.Requester,
		Total:     state.Total,
	}).Get(ctx, nil)

	// Step 3: Wait for manager decision (approve or reject) with 1h timeout
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timerFuture := workflow.NewTimer(timerCtx, 1*time.Hour)

	for decision == "" {
		drainSignals()
		if decision != "" {
			break
		}
		sel := workflow.NewSelector(ctx)
		sel.AddReceive(approveCh, func(c workflow.ReceiveChannel, more bool) {
			var note string
			c.Receive(ctx, &note)
			recordApprove(note)
		})
		sel.AddReceive(rejectCh, func(c workflow.ReceiveChannel, more bool) {
			var note string
			c.Receive(ctx, &note)
			recordReject(note)
		})
		sel.AddFuture(timerFuture, func(f workflow.Future) {
			if decision == "" {
				decision = "rejected"
				decisionNote = "Auto-rejected: approval timed out after 1 hour"
			}
		})
		sel.Select(ctx)
	}
	cancelTimer()

	// Step 4: Process decision and update both surfaces
	state.Status = decision
	state.DecisionNote = decisionNote

	_ = workflow.ExecuteActivity(actCtx, acts.ProcessDecision, ProcessDecisionReq{
		RequestID: state.RequestID,
		Decision:  state.Status,
		Note:      state.DecisionNote,
	}).Get(ctx, nil)

	publishRequester(UpdateRequesterStatus(state))
	publishApprover(UpdateApproverDecision(state))

	return state.Status, nil
}
