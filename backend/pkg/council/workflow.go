package council

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

// CouncilWorkflow demonstrates two advanced production patterns for students:
//  1. Option A (Parallel Multi-Agent Fan-Out): Using workflow.Go to dispatch 3 concurrent
//     remote A2A specialist evaluations and progressively stream each A2UI card as it finishes.
//  2. Option C (Live Schema / Layout Migration): Using workflow.GetVersion + a live
//     "upgrade_schema" signal to hot-upgrade a running workflow's A2UI surface from v1 to v2
//     without losing state or breaking deterministic replay.
func CouncilWorkflow(ctx workflow.Context, input CouncilInput) (string, error) {
	input.ApplyDefaults()

	stream, err := a2ui.NewStream(ctx, input.StreamState)
	if err != nil {
		return "", err
	}
	ui := stream.Topic("a2ui")

	publishA2UI := func(msgs []map[string]any) {
		for _, m := range msgs {
			_ = ui.Publish(m)
		}
	}
	publishTrace := func(step string, detail map[string]any) {
		_ = ui.Publish(map[string]any{
			"trace": map[string]any{
				"step":   step,
				"detail": detail,
			},
		})
	}
	publishChat := func(role, text string) {
		_ = ui.Publish(map[string]any{
			"chat": map[string]any{
				"role": role,
				"text": text,
			},
		})
	}

	// Option C: Deterministic versioning via workflow.GetVersion.
	// New executions default to version 2 capability while supporting live v1 -> v2 toggle for teaching.
	recordedVersion := workflow.GetVersion(ctx, "council-verdict-schema", workflow.DefaultVersion, 2)
	activeSchema := input.TargetSchema
	if recordedVersion == workflow.DefaultVersion {
		activeSchema = 1
	}

	state := CouncilState{
		ProposalTitle: input.ProposalTitle,
		Architect:     input.Architect,
		Service:       input.Service,
		SchemaVersion: activeSchema,
		Completed:     0,
		TotalBranches: 3,
	}

	// Step 1: Create initial progressive board surface (0 / 3 specialists completed)
	boardInit := BuildCouncilBoardMessages(state, true)
	publishA2UI(boardInit)
	publishChat("system", fmt.Sprintf("⚡ Launching 3 parallel A2A specialist coroutines via `workflow.Go` for **%s** (`schema v%d`)...", state.ProposalTitle, state.SchemaVersion))
	publishTrace("fanout_started", map[string]any{
		"pattern":        "workflow.Go Parallel A2A Fan-Out",
		"branches":       []string{"SRE Telemetry (:9003)", "SecOps Policy (:9004)", "FinOps Advisor (:9001)"},
		"schema_version": state.SchemaVersion,
		"a2ui_payloads":  boardInit,
	})

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 1.5,
			MaximumInterval:    3 * time.Second,
			MaximumAttempts:    20,
		},
	})
	var acts *Activities
	req := AuditRequest{
		Service:       state.Service,
		ProposalTitle: state.ProposalTitle,
	}

	// Step 2: Fan out all 3 A2A specialist calls concurrently using workflow.Go
	branchCh := workflow.NewChannel(ctx)

	workflow.Go(ctx, func(gCtx workflow.Context) {
		var res SpecialistResult
		err := workflow.ExecuteActivity(actCtx, acts.AuditReliabilityA2A, req).Get(gCtx, &res)
		if err != nil {
			res = SpecialistResult{Domain: "reliability", Status: "ERROR", Headline: err.Error()}
		}
		branchCh.Send(gCtx, res)
	})

	workflow.Go(ctx, func(gCtx workflow.Context) {
		var res SpecialistResult
		err := workflow.ExecuteActivity(actCtx, acts.AuditSecOpsA2A, req).Get(gCtx, &res)
		if err != nil {
			res = SpecialistResult{Domain: "secops", Status: "ERROR", Headline: err.Error()}
		}
		branchCh.Send(gCtx, res)
	})

	workflow.Go(ctx, func(gCtx workflow.Context) {
		var res SpecialistResult
		err := workflow.ExecuteActivity(actCtx, acts.AuditFinOpsA2A, req).Get(gCtx, &res)
		if err != nil {
			res = SpecialistResult{Domain: "finops", Status: "ERROR", Headline: err.Error()}
		}
		branchCh.Send(gCtx, res)
	})

	// Step 3: Progressively stream each specialist card onto the A2UI surface as it arrives
	for i := 0; i < 3; i++ {
		var res SpecialistResult
		branchCh.Receive(ctx, &res)
		state.Completed++
		switch res.Domain {
		case "reliability":
			r := res
			state.Reliability = &r
		case "secops":
			r := res
			state.SecOps = &r
		case "finops":
			r := res
			state.FinOps = &r
		}

		updateMsgs := BuildCouncilBoardMessages(state, false)
		publishA2UI(updateMsgs)
		publishChat("assistant", fmt.Sprintf("✅ **%s** finished (%d/3): *%s* — `%s` (Attempt #%d)",
			res.AgentName, state.Completed, res.Headline, res.KeyMetric, res.XRay.Attempt))
		publishTrace("a2a_branch_completed", map[string]any{
			"domain":        res.Domain,
			"agent":         res.AgentName,
			"endpoint":      res.Endpoint,
			"key_metric":    res.KeyMetric,
			"attempt":       res.XRay.Attempt,
			"completed":     state.Completed,
			"total":         state.TotalBranches,
			"xray":          res.XRay,
			"a2ui_payloads": updateMsgs,
		})
	}

	// Step 4: Render the Verdict Surface using the active schema version (v1 or v2)
	renderVerdict := func(create bool) []map[string]any {
		if state.SchemaVersion >= 2 {
			return BuildVerdictSurfaceV2(state, create)
		}
		return BuildVerdictSurfaceV1(state, create)
	}

	verdictMsgs := renderVerdict(true)
	publishA2UI(verdictMsgs)
	publishTrace("verdict_surface_rendered", map[string]any{
		"version_id":     "council-verdict-schema",
		"schema_version": state.SchemaVersion,
		"a2ui_payloads":  verdictMsgs,
	})

	// Step 5: Listen for live schema upgrade ("upgrade_schema") OR final council decision ("council_decision")
	upgradeCh := workflow.GetSignalChannel(ctx, "upgrade_schema")
	decisionCh := workflow.GetSignalChannel(ctx, "council_decision")
	userActionCh := workflow.GetSignalChannel(ctx, "user_action")

	applySchemaUpgrade := func(target int) {
		if target < 1 || target > 2 || target == state.SchemaVersion {
			return
		}
		prev := state.SchemaVersion
		state.SchemaVersion = target
		boardUpdate := BuildCouncilBoardMessages(state, false)
		verdictUpdate := renderVerdict(false)
		publishA2UI(boardUpdate)
		publishA2UI(verdictUpdate)
		publishChat("system", fmt.Sprintf("🔼 **Live Schema Migration (`workflow.GetVersion`)**: Hot-swapped `%s` from **v%d** → **v%d** with zero downtime or lost state!", VerdictSurface, prev, state.SchemaVersion))
		publishTrace("schema_migrated", map[string]any{
			"change_id":     "council-verdict-schema",
			"from_version":  prev,
			"to_version":    state.SchemaVersion,
			"a2ui_payloads": verdictUpdate,
		})
	}

	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timerFuture := workflow.NewTimer(timerCtx, 1*time.Hour)

	for state.Decision == "" {
		sel := workflow.NewSelector(ctx)
		sel.AddReceive(upgradeCh, func(c workflow.ReceiveChannel, more bool) {
			var target int
			c.Receive(ctx, &target)
			if target == 0 {
				if state.SchemaVersion == 1 {
					target = 2
				} else {
					target = 1
				}
			}
			applySchemaUpgrade(target)
		})
		sel.AddReceive(decisionCh, func(c workflow.ReceiveChannel, more bool) {
			var dec string
			c.Receive(ctx, &dec)
			if dec == "" {
				dec = "approved"
			}
			state.Decision = dec
		})
		sel.AddReceive(userActionCh, func(c workflow.ReceiveChannel, more bool) {
			var act map[string]any
			c.Receive(ctx, &act)
			name, _ := act["name"].(string)
			ctxMap, _ := act["context"].(map[string]any)
			switch name {
			case "upgrade_schema":
				target := 2
				if t, ok := ctxMap["target"].(float64); ok && t >= 1 {
					target = int(t)
				}
				applySchemaUpgrade(target)
			case "council_decision":
				dec, _ := ctxMap["decision"].(string)
				if dec == "" {
					dec = "approved"
				}
				state.Decision = dec
			}
		})
		sel.AddFuture(timerFuture, func(f workflow.Future) {
			if state.Decision == "" {
				state.Decision = "changes_requested"
			}
		})
		sel.Select(ctx)
	}
	cancelTimer()

	finalMsgs := renderVerdict(false)
	publishA2UI(finalMsgs)
	publishChat("system", fmt.Sprintf("🏁 Architecture Council finalized decision: **%s** (rendered on Schema **v%d**).", state.Decision, state.SchemaVersion))
	publishTrace("council_completed", map[string]any{
		"decision":       state.Decision,
		"schema_version": state.SchemaVersion,
		"a2ui_payloads":  finalMsgs,
	})

	return state.Decision, nil
}
