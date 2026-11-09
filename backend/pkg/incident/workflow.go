package incident

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/contrib/workflowstreams"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

var SREDemoMessages = []string{
	"Investigate SEV-1 telemetry and canary diff for checkout-api",
	"Evaluate blast radius and SecOps policy",
	"Execute canary rollback RB-402 now!",
}

func IncidentWorkflow(ctx workflow.Context, input IncidentInput) (string, error) {
	input.ApplyDefaults()

	stream, err := workflowstreams.NewWorkflowStream(ctx, input.StreamState)
	if err != nil {
		return "", err
	}
	ui := stream.Topic("a2ui")

	commander := input.CommanderName
	pendingMessages := append([]string(nil), input.PendingMessages...)
	sessionEnded := false
	messageCounter := input.MessageCounter
	telemetryChecked := input.TelemetryChecked
	policyChecked := input.PolicyChecked
	remediated := input.Remediated
	totalTokens := input.TotalTokens
	totalCostUSD := input.TotalCostUSD
	turnCount := input.TurnCount
	toolCallCount := input.ToolCallCount
	approvalRequired := input.ApprovalRequired
	approvalDecision := input.ApprovalDecision
	traceSeq := 0

	var queuedA2UI []map[string]any

	publishChat := func(role, text string) {
		messageCounter++
		msgID := fmt.Sprintf("msg-%d", messageCounter)
		ui.Publish(a2ui.UpdateDataModel("chat", fmt.Sprintf("/messages/%s", msgID), map[string]any{
			"role": role,
			"text": text,
			"id":   msgID,
		}))
	}

	streamText := func(msgID, text string, streaming bool) {
		ui.Publish(a2ui.UpdateDataModel("chat", fmt.Sprintf("/messages/%s", msgID), map[string]any{
			"role":      "assistant",
			"text":      text,
			"id":        msgID,
			"streaming": streaming,
		}))
	}

	setTyping := func(typing bool) {
		ui.Publish(a2ui.UpdateDataModel("chat", "/typing", typing))
	}

	publishTrace := func(eventType string, fields map[string]any) {
		traceSeq++
		traceID := fmt.Sprintf("trace-%d", traceSeq)
		val := map[string]any{
			"id":        traceID,
			"type":      eventType,
			"timestamp": float64(workflow.Now(ctx).UnixNano()) / 1e9,
		}
		for k, v := range fields {
			val[k] = v
		}
		ui.Publish(map[string]any{
			"updateDataModel": map[string]any{
				"path":  fmt.Sprintf("/trace/%s", traceID),
				"value": val,
			},
		})
	}

	publishMetrics := func() {
		ui.Publish(map[string]any{
			"updateDataModel": map[string]any{
				"path": "/metrics",
				"value": map[string]any{
					"total_tokens":    totalTokens,
					"total_cost_usd":  totalCostUSD,
					"turn_count":      turnCount,
					"tool_call_count": toolCallCount,
				},
			},
		})
	}

	publishApprovalBanner := func(status string) {
		ui.Publish(map[string]any{
			"updateDataModel": map[string]any{
				"path": "/approval",
				"value": map[string]any{
					"status":        status,
					"commander":     commander,
					"service":       input.ServiceName,
					"runbook":       "RB-402: Shift 100% GKE traffic to checkout-api-stable-v2.13.8 & drain leaked AlloyDB tx",
					"blast_radius":  "14,200 req/s across mx-central1 & us-central1 (Change Freeze Active)",
					"slo_burn_rate": "18.4x",
				},
			},
		})
	}

	userMsgCh := workflow.GetSignalChannel(ctx, "user_message")
	userActionCh := workflow.GetSignalChannel(ctx, "user_action")
	ffCh := workflow.GetSignalChannel(ctx, "fast_forward")
	cmdDecisionCh := workflow.GetSignalChannel(ctx, "commander_decision")
	endCh := workflow.GetSignalChannel(ctx, "end_session")

	drainSignals := func() {
		for {
			var txt string
			if userMsgCh.ReceiveAsync(&txt) {
				if strings.TrimSpace(txt) != "" {
					pendingMessages = append(pendingMessages, txt)
				}
				continue
			}
			var act map[string]any
			if userActionCh.ReceiveAsync(&act) {
				evName, _ := act["name"].(string)
				if evName == "" {
					evName, _ = act["event"].(string)
				}
				if evName != "" {
					pendingMessages = append(pendingMessages, evName)
				}
				continue
			}
			var empty any
			if ffCh.ReceiveAsync(&empty) {
				idx := turnCount
				if idx >= len(SREDemoMessages) {
					idx = len(SREDemoMessages) - 1
				}
				pendingMessages = append(pendingMessages, SREDemoMessages[idx])
				continue
			}
			var dec string
			if cmdDecisionCh.ReceiveAsync(&dec) {
				if dec == "approved" || dec == "rejected" {
					approvalDecision = dec
				}
				continue
			}
			if endCh.ReceiveAsync(&empty) {
				sessionEnded = true
				continue
			}
			break
		}
	}

	hooks := &IncidentToolHooks{
		OnExecuteRemediation: func(args ExecuteRemediationArgs) (ExecuteRemediationResult, error) {
			// 1. PROGRAMMATIC SECOPS GUARDRAIL
			if !policyChecked {
				publishChat("system", "🛡️ *Programmatic SecOps Guardrail: Blocking `execute_remediation` until A2A Blast-Radius & Change-Freeze Policy (`:9004`) is verified…*")
				publishTrace("function_response", map[string]any{"function": "execute_remediation", "status": "blocked"})
				return ExecuteRemediationResult{
					Status:  "blocked",
					Reason:  "secops_policy_check_required",
					Message: "SAFETY: Cannot execute production canary rollback before running a2a_evaluate_blast_radius.",
				}, nil
			}

			// 2. INCIDENT COMMANDER HITL APPROVAL GATE
			if !approvalRequired && approvalDecision != "approved" {
				approvalRequired = true
				approvalDecision = ""
				publishChat("system", "🚨 *Tier-0 Change-Freeze Gate: Waiting for Incident Commander HITL authorization before executing Runbook RB-402…*")
				publishApprovalBanner("pending")
				publishTrace("approval_gate", map[string]any{"runbook": "RB-402", "service": input.ServiceName})
				return ExecuteRemediationResult{
					Status:  "pending_approval",
					Runbook: "RB-402",
					Message: "Production traffic shift during Peak Event Change Freeze requires Incident Commander approval.",
				}, nil
			}

			remediated = true
			queuedA2UI = append(queuedA2UI, BuildRemediationSurface(commander)...)
			publishTrace("function_response", map[string]any{"function": "execute_remediation", "status": "allowed"})
			return ExecuteRemediationResult{
				Status:  "executed",
				Runbook: "RB-402",
				Message: "Canary rollback RB-402 complete. Traffic shifted to checkout-api-stable-v2.13.8.",
			}, nil
		},
	}

	rootAgent, err := CreateIncidentAgent(hooks)
	if err != nil {
		return "", err
	}

	adkCtx := googleadk.NewContext(ctx)
	svc := session.InMemoryService()
	sessionID := "sre-session-1"
	if input.SessionSnapshot != nil {
		_, _ = googleadk.ImportSession(adkCtx, svc, input.SessionSnapshot)
	}

	r, err := runner.New(runner.Config{
		AppName:           "cloudguard-sre",
		Agent:             rootAgent,
		SessionService:    svc,
		AutoCreateSession: true,
	})
	if err != nil {
		return "", err
	}

	if input.StreamState == nil {
		ui.Publish(a2ui.CreateSurface("chat", false))
		publishChat("assistant", fmt.Sprintf("🚨 **CloudGuard AI SRE Incident Commander** active for Commander **%s**.\n\n**Active Alert:** `%s` — *%s*\n\nClick a prompt below or press **Fast Forward (`F`)** to run the multi-agent A2A diagnostic & remediation pipeline.", commander, input.ServiceName, input.IncidentSummary))
	}

	for !sessionEnded {
		drainSignals()
		for len(pendingMessages) == 0 && !sessionEnded {
			_ = workflow.Await(ctx, func() bool {
				drainSignals()
				return len(pendingMessages) > 0 || sessionEnded
			})
		}
		if sessionEnded {
			break
		}

		msg := pendingMessages[0]
		pendingMessages = pendingMessages[1:]

		publishChat("user", msg)
		setTyping(true)

		turnCount++
		tokensThisTurn := 0
		publishTrace("turn_start", map[string]any{"message": msg, "turn": turnCount})

		messageCounter++
		streamMsgID := fmt.Sprintf("msg-%d", messageCounter)
		responseText := ""
		currentAgent := "incident_commander"
		queuedA2UI = nil

		userContent := genai.NewContentFromText(msg, genai.RoleUser)
		for ev, runErr := range r.Run(adkCtx, commander, sessionID, userContent, agent.RunConfig{}) {
			if runErr != nil {
				workflow.GetLogger(ctx).Error("SRE ADK runner error", "error", runErr)
				responseText = "Temporary error in SRE agent pipeline. Please retry."
				break
			}
			if ev == nil {
				continue
			}

			if ev.Author != "" && ev.Author != currentAgent {
				currentAgent = ev.Author
				agentLabels := map[string]string{
					"telemetry_investigator": "📡 Telemetry Investigator",
					"mitigation_planner":     "🛡️ Mitigation Planner",
					"remediation_executor":   "🚀 Remediation Executor",
				}
				if label, ok := agentLabels[currentAgent]; ok {
					publishChat("system", fmt.Sprintf("*Transferring to %s…*", label))
					publishTrace("agent_handoff", map[string]any{"agent": label})
					messageCounter++
					streamMsgID = fmt.Sprintf("msg-%d", messageCounter)
				}
			}

			if ev.UsageMetadata != nil && ev.UsageMetadata.TotalTokenCount > 0 {
				tks := int(ev.UsageMetadata.TotalTokenCount)
				totalTokens += tks
				tokensThisTurn += tks
				totalCostUSD += (float64(tks) / 1_000_000.0) * 0.25
				publishMetrics()
			}

			if ev.Content != nil {
				for _, part := range ev.Content.Parts {
					if part == nil {
						continue
					}
					if part.Text != "" {
						responseText += part.Text
						streamText(streamMsgID, responseText, true)
					}
					if part.FunctionCall != nil {
						fnName := part.FunctionCall.Name
						toolCallCount++
						if strings.HasPrefix(fnName, "a2a_") {
							pretty := strings.Title(strings.ReplaceAll(strings.TrimPrefix(fnName, "a2a_"), "_", " "))
							publishChat("system", fmt.Sprintf("🔗 *Calling %s A2A agent over JSON-RPC 2.0…*", pretty))
							publishTrace("tool_call", map[string]any{"tool": fnName})
							messageCounter++
							streamMsgID = fmt.Sprintf("msg-%d", messageCounter)
							responseText = ""
						} else if fnName != "transfer_to_agent" {
							publishTrace("tool_call", map[string]any{"tool": fnName})
						}
					}
					if part.FunctionResponse != nil {
						fnName := part.FunctionResponse.Name
						if fnName == "a2a_diagnose_telemetry" {
							telemetryChecked = true
							publishTrace("function_response", map[string]any{"function": fnName, "status": "allowed"})
							queuedA2UI = append(queuedA2UI, BuildTelemetrySurface()...)
						} else if fnName == "a2a_evaluate_blast_radius" {
							policyChecked = true
							publishTrace("function_response", map[string]any{"function": fnName, "status": "allowed"})
							queuedA2UI = append(queuedA2UI, BuildRunbookSurface()...)
						}
					}
				}
			}
		}

		setTyping(false)
		publishTrace("turn_end", map[string]any{"turn": turnCount, "tokens_this_turn": tokensThisTurn})
		publishMetrics()

		if responseText != "" {
			streamText(streamMsgID, responseText, false)
		}
		for _, a2uiMsg := range queuedA2UI {
			ui.Publish(a2uiMsg)
		}

		// Wait for Incident Commander HITL approval if triggered
		if approvalRequired && approvalDecision == "" {
			publishChat("system", "⏳ *Waiting for Incident Commander sign-off (5m timeout)…*")
			ok, _ := workflow.AwaitWithTimeout(ctx, 5*time.Minute, func() bool {
				drainSignals()
				return approvalDecision != "" || sessionEnded
			})
			if !ok && approvalDecision == "" {
				approvalDecision = "timeout"
				approvalRequired = false
				publishApprovalBanner("rejected")
				publishTrace("approval_resolved", map[string]any{"decision": "timeout"})
				pendingMessages = append([]string{"[Commander approval timed out — rollback aborted.]"}, pendingMessages...)
			} else if approvalDecision == "approved" {
				publishChat("system", "✅ *Incident Commander authorized Runbook RB-402!*")
				approvalRequired = false
				publishApprovalBanner("approved")
				publishTrace("approval_resolved", map[string]any{"decision": "approved"})
				pendingMessages = append([]string{"[Commander approved Runbook RB-402. Execute canary rollback now!]"}, pendingMessages...)
			} else if approvalDecision == "rejected" {
				publishChat("system", "🛑 *Incident Commander aborted Runbook RB-402.*")
				approvalRequired = false
				publishApprovalBanner("rejected")
				publishTrace("approval_resolved", map[string]any{"decision": "rejected"})
				pendingMessages = append([]string{"[Commander rejected the rollback.]"}, pendingMessages...)
			}
		}

		_ = telemetryChecked
		_ = remediated
	}

	return "incident_closed", nil
}
