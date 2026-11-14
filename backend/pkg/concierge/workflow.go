package concierge

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
	"github.com/ab/durable-a2ui/backend/pkg/order"
)

const MaxMessagesBeforeCompaction = 500

var DemoMessages = []string{
	"What's on the menu?",
	"Order 4 Tacos al Pastor and 2 Horchata Artesanal",
	"Also add 2 Brown Sugar Boba",
	"What's in my cart?",
	"Confirm my order!",
	"What's my order status?",
	"Thanks!",
}

func ConciergeWorkflow(ctx workflow.Context, input ConciergeInput) (string, error) {
	input.ApplyDefaults()

	stream, err := workflowstreams.NewWorkflowStream(ctx, input.StreamState)
	if err != nil {
		return "", err
	}
	ui := stream.Topic("a2ui")

	customer := input.CustomerName
	pendingMessages := append([]string(nil), input.PendingMessages...)
	sessionEnded := false
	messageCounter := input.MessageCounter
	orderStarted := input.OrderStarted
	var orderUpdates []string
	confirmedItems := append([]CartItem(nil), input.ConfirmedItems...)
	cart := make(map[string]CartItem, len(input.Cart))
	for k, v := range input.Cart {
		cart[k] = v
	}
	dietaryChecked := input.DietaryChecked
	totalTokens := input.TotalTokens
	totalCostUSD := input.TotalCostUSD
	turnCount := input.TurnCount
	toolCallCount := input.ToolCallCount
	approvalThreshold := input.ApprovalThreshold
	approvalRequired := input.ApprovalRequired
	approvalDecision := input.ApprovalDecision
	approvalTimeout := time.Duration(input.ApprovalTimeoutSeconds) * time.Second
	traceSeq := 0

	var queuedA2UI []map[string]any

	// Publish helpers
	publishChat := func(role, text string) {
		messageCounter++
		msgID := fmt.Sprintf("msg-%d", messageCounter)
		_ = ui.Publish(a2ui.UpdateDataModel("chat", fmt.Sprintf("/messages/%s", msgID), map[string]any{
			"role": role,
			"text": text,
			"id":   msgID,
		}))
	}

	streamText := func(msgID, text string, streaming bool) {
		_ = ui.Publish(a2ui.UpdateDataModel("chat", fmt.Sprintf("/messages/%s", msgID), map[string]any{
			"role":      "assistant",
			"text":      text,
			"id":        msgID,
			"streaming": streaming,
		}))
	}

	setTyping := func(typing bool) {
		_ = ui.Publish(a2ui.UpdateDataModel("chat", "/typing", typing))
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
		_ = ui.Publish(a2ui.UpdateDataModel("chat", fmt.Sprintf("/trace/%s", traceID), val))
	}

	publishMetrics := func() {
		_ = ui.Publish(a2ui.UpdateDataModel("chat", "/metrics", map[string]any{
			"total_tokens":    totalTokens,
			"total_cost_usd":  totalCostUSD,
			"turn_count":      turnCount,
			"tool_call_count": toolCallCount,
		}))
	}

	publishApprovalCard := func(items []CartItem, total float64) {
		parts := make([]string, 0, len(items))
		for _, i := range items {
			parts = append(parts, fmt.Sprintf("%s ×%d", i.Name, i.Qty))
		}
		_ = ui.Publish(a2ui.UpdateDataModel("chat", "/approval", map[string]any{
			"status":    "pending",
			"total":     total,
			"threshold": approvalThreshold,
			"items":     strings.Join(parts, ", "),
			"customer":  customer,
		}))
	}

	publishApprovalUpdate := func(decision string) {
		_ = ui.Publish(a2ui.UpdateDataModel("chat", "/approval", map[string]any{
			"status":   decision,
			"customer": customer,
		}))
	}

	// Signal channels
	userMsgCh := workflow.GetSignalChannel(ctx, "user_message")
	userActionCh := workflow.GetSignalChannel(ctx, "user_action")
	ffCh := workflow.GetSignalChannel(ctx, "fast_forward")
	mgrDecisionCh := workflow.GetSignalChannel(ctx, "manager_decision")
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
				if evName != "" && !strings.HasPrefix(evName, "track_order:") {
					pendingMessages = append(pendingMessages, evName)
				}
				continue
			}
			var empty any
			if ffCh.ReceiveAsync(&empty) {
				idx := turnCount
				if idx >= len(DemoMessages) {
					idx = len(DemoMessages) - 1
				}
				pendingMessages = append(pendingMessages, DemoMessages[idx])
				continue
			}
			var dec string
			if mgrDecisionCh.ReceiveAsync(&dec) {
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

	// Deterministic in-workflow tool hooks
	hooks := &WorkflowToolHooks{
		OnGetMenu: func(args GetMenuArgs) (GetMenuResult, error) {
			queuedA2UI = append(queuedA2UI, BuildMenuSurface(args.Category)...)
			return GetMenuResult{
				Restaurant: "Durable Eats (Local & Campus)",
				Items:      MenuCatalog,
			}, nil
		},
		OnAddToCart: func(args AddToCartArgs) (AddToCartResult, error) {
			qty := args.Quantity
			if qty <= 0 {
				qty = 1
			}
			if qty > 99 {
				return AddToCartResult{Error: "Quantity must be between 1 and 99"}, nil
			}
			item, ok := FindMenuItem(args.ItemName)
			if !ok {
				return AddToCartResult{Error: fmt.Sprintf("Item %q is not on the menu", args.ItemName)}, nil
			}
			existing, exists := cart[item.Name]
			if exists {
				existing.Qty += qty
				cart[item.Name] = existing
			} else {
				cart[item.Name] = CartItem{Name: item.Name, Qty: qty, Price: item.Price}
			}
			queuedA2UI = append(queuedA2UI, BuildCartSurface(cart)...)
			return AddToCartResult{
				Added:     item.Name,
				Quantity:  qty,
				UnitPrice: item.Price,
				Subtotal:  item.Price * float64(qty),
			}, nil
		},
		OnRemoveCart: func(args RemoveFromCartArgs) (RemoveFromCartResult, error) {
			name := args.ItemName
			if item, ok := FindMenuItem(args.ItemName); ok {
				name = item.Name
			}
			if existing, ok := cart[name]; ok {
				existing.Qty--
				if existing.Qty <= 0 {
					delete(cart, name)
				} else {
					cart[name] = existing
				}
			}
			queuedA2UI = append(queuedA2UI, BuildCartSurface(cart)...)
			return RemoveFromCartResult{Removed: name, Quantity: 1}, nil
		},
		OnGetCart: func() (GetCartResult, error) {
			items := make([]CartItem, 0, len(cart))
			var total float64
			var count int
			for _, item := range cart {
				items = append(items, item)
				total += item.Price * float64(item.Qty)
				count += item.Qty
			}
			queuedA2UI = append(queuedA2UI, BuildCartSurface(cart)...)
			return GetCartResult{Items: items, Total: total, ItemCount: count}, nil
		},
		OnConfirmOrder: func(args ConfirmOrderArgs) (ConfirmOrderResult, error) {
			// 1. PROGRAMMATIC SAFETY GUARDRAIL
			if len(cart) > 0 && !dietaryChecked {
				publishChat("system", "🛡️ *Programmatic Guardrail: Blocking `confirm_order` until A2A Dietary Safety check (`:9002`) completes…*")
				publishTrace("function_response", map[string]any{"function": "confirm_order", "status": "blocked"})
				return ConfirmOrderResult{
					Status: "blocked",
					Reason: "dietary_check_required",
					Message: "SAFETY: Cannot confirm order — dietary safety check has not been completed. " +
						"Please call a2a_check_dietary_safety first to verify all items are safe before confirming.",
				}, nil
			}

			items := make([]CartItem, 0, len(cart))
			var total float64
			for _, item := range cart {
				items = append(items, item)
				total += item.Price * float64(item.Qty)
			}
			if len(items) == 0 {
				items = []CartItem{{Name: "Tacos al Pastor", Qty: 2, Price: 11.50}}
				total = 23.00
			}

			// 2. HUMAN-IN-THE-LOOP MANAGER APPROVAL GATE
			if total > approvalThreshold && !approvalRequired && approvalDecision != "approved" {
				approvalRequired = true
				approvalDecision = ""
				publishChat("system", fmt.Sprintf("💼 *Order requires manager approval ($%.2f exceeds $%.2f limit)…*", total, approvalThreshold))
				publishApprovalCard(items, total)
				publishTrace("approval_gate", map[string]any{"total": total, "threshold": approvalThreshold})
				return ConfirmOrderResult{
					Status:    "pending_approval",
					Total:     total,
					Threshold: approvalThreshold,
					Message:   fmt.Sprintf("Order total $%.2f exceeds $%.2f limit. Waiting for manager approval before confirming.", total, approvalThreshold),
				}, nil
			}

			confirmedItems = items
			publishTrace("function_response", map[string]any{"function": "confirm_order", "status": "allowed"})
			return ConfirmOrderResult{
				Status:   "confirmed",
				Customer: customer,
				Items:    items,
				Total:    total,
				Message:  fmt.Sprintf("Order confirmed! Total: $%.2f. Your food is being prepared.", total),
			}, nil
		},
		OnSendA2UI: func(args SendA2UIArgs) (SendA2UIResult, error) {
			msgs, err := a2ui.ParseAndValidate(args.A2UIJSON)
			if err != nil {
				return SendA2UIResult{Valid: false, Error: err.Error()}, nil
			}
			queuedA2UI = append(queuedA2UI, msgs...)
			return SendA2UIResult{Valid: true, Count: len(msgs), A2UIMsg: msgs}, nil
		},
	}

	rootAgent, err := CreateConciergeAgent(hooks)
	if err != nil {
		return "", err
	}

	adkCtx := googleadk.NewContext(ctx)
	svc := session.InMemoryService()
	sessionID := "session-1"
	if input.SessionSnapshot != nil {
		if _, err := googleadk.ImportSession(adkCtx, svc, input.SessionSnapshot); err != nil {
			workflow.GetLogger(ctx).Warn("Failed to import session snapshot", "error", err)
		}
	}

	r, err := runner.New(runner.Config{
		AppName:           "durable-eats",
		Agent:             rootAgent,
		SessionService:    svc,
		AutoCreateSession: true,
	})
	if err != nil {
		return "", err
	}

	// Create initial chat surface and greeting (only on first run before compaction)
	if input.StreamState == nil {
		_ = ui.Publish(a2ui.CreateSurface("chat", false))
		publishChat("assistant", fmt.Sprintf("Hi %s! 👋 I'm your AI concierge at **Durable Eats** (powered by Go ADK + A2A + A2UI + Temporal). Want to see our **Taquería El Sol & Campus Boba** menu or order for your team?", customer))
	}

	for !sessionEnded {
		// Wait for next user message or drip an order update
		drainSignals()
		for len(pendingMessages) == 0 && !sessionEnded {
			if len(orderUpdates) > 0 {
				ok, _ := workflow.AwaitWithTimeout(ctx, 15*time.Second, func() bool {
					drainSignals()
					return len(pendingMessages) > 0 || sessionEnded
				})
				if !ok && len(orderUpdates) > 0 {
					nextUpdate := orderUpdates[0]
					orderUpdates = orderUpdates[1:]
					publishChat("assistant", nextUpdate)
				}
			} else {
				_ = workflow.Await(ctx, func() bool {
					drainSignals()
					return len(pendingMessages) > 0 || sessionEnded
				})
			}
		}
		if sessionEnded {
			break
		}

		msg := pendingMessages[0]
		pendingMessages = pendingMessages[1:]

		publishChat("user", msg)
		setTyping(true)

		// Process one ADK agent turn
		turnCount++
		tokensThisTurn := 0
		publishTrace("turn_start", map[string]any{"message": msg, "turn": turnCount})

		messageCounter++
		streamMsgID := fmt.Sprintf("msg-%d", messageCounter)
		responseText := ""
		currentAgent := "concierge"
		queuedA2UI = nil

		userContent := genai.NewContentFromText(msg, genai.RoleUser)
		for ev, runErr := range r.Run(adkCtx, customer, sessionID, userContent, agent.RunConfig{}) {
			if runErr != nil {
				workflow.GetLogger(ctx).Error("ADK runner error", "error", runErr)
				responseText = "I encountered a temporary issue processing that request. Please try again."
				break
			}
			if ev == nil {
				continue
			}

			// 1. Track multi-agent handoffs
			if ev.Author != "" && ev.Author != currentAgent {
				currentAgent = ev.Author
				agentLabels := map[string]string{
					"menu_advisor":    "📋 Menu Advisor",
					"cart_manager":    "🛒 Cart Manager",
					"order_confirmer": "✅ Order Confirmer",
				}
				if label, ok := agentLabels[currentAgent]; ok {
					publishChat("system", fmt.Sprintf("*Transferring to %s…*", label))
					publishTrace("agent_handoff", map[string]any{"agent": label})
					messageCounter++
					streamMsgID = fmt.Sprintf("msg-%d", messageCounter)
				}
			}

			// 2. Track token usage
			if ev.UsageMetadata != nil && ev.UsageMetadata.TotalTokenCount > 0 {
				tks := int(ev.UsageMetadata.TotalTokenCount)
				totalTokens += tks
				tokensThisTurn += tks
				totalCostUSD += (float64(tks) / 1_000_000.0) * 0.25
				publishMetrics()
			}

			// 3. Process content parts (text, function calls, function responses)
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
							pretty := strings.ReplaceAll(strings.TrimPrefix(fnName, "a2a_"), "_", " ")
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
						if fnName == "a2a_check_dietary_safety" {
							dietaryChecked = true
							publishTrace("function_response", map[string]any{"function": fnName, "status": "allowed"})
							queuedA2UI = append(queuedA2UI, BuildDietarySurface("All cart items verified safe against our allergen database (nut-free kitchen certified, gluten-free corn tortillas verified).")...)
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
			_ = ui.Publish(a2uiMsg)
		}

		// Human-in-the-loop: wait for manager approval if triggered
		if approvalRequired && approvalDecision == "" {
			mins := int(approvalTimeout.Minutes())
			publishChat("system", fmt.Sprintf("⏳ *Waiting for manager approval (%dm timeout)…*", mins))
			ok, _ := workflow.AwaitWithTimeout(ctx, approvalTimeout, func() bool {
				drainSignals()
				return approvalDecision != "" || sessionEnded
			})
			if !ok && approvalDecision == "" {
				approvalDecision = "timeout"
				publishChat("system", fmt.Sprintf("⏰ *Approval timed out after %d minutes — order auto-cancelled.*", mins))
				approvalRequired = false
				publishApprovalUpdate("rejected")
				publishTrace("approval_resolved", map[string]any{"decision": "timeout"})
				pendingMessages = append([]string{"[Manager did not respond in time. The order has been auto-cancelled.]"}, pendingMessages...)
			} else if approvalDecision == "approved" {
				publishChat("system", "✅ *Manager approved the order!*")
				approvalRequired = false
				publishApprovalUpdate("approved")
				publishTrace("approval_resolved", map[string]any{"decision": "approved"})
				pendingMessages = append([]string{"[Manager approved the order. Please confirm it now.]"}, pendingMessages...)
			} else if approvalDecision == "rejected" {
				publishChat("system", "❌ *Manager rejected the order.*")
				approvalRequired = false
				publishApprovalUpdate("rejected")
				publishTrace("approval_resolved", map[string]any{"decision": "rejected"})
				pendingMessages = append([]string{"[Manager rejected the order. Let the customer know politely.]"}, pendingMessages...)
			}
		}

		// Start OrderTracking Child Workflow once confirmed
		if !orderStarted && len(confirmedItems) > 0 {
			orderStarted = true
			orderItems := make([]order.OrderItem, 0, len(confirmedItems))
			var total float64
			var summaryParts []string
			for _, ci := range confirmedItems {
				orderItems = append(orderItems, order.OrderItem{
					Name:  ci.Name,
					Qty:   ci.Qty,
					Price: ci.Price,
				})
				total += ci.Price * float64(ci.Qty)
				summaryParts = append(summaryParts, fmt.Sprintf("%d× %s", ci.Qty, ci.Name))
			}

			var randBytes [4]byte
			_, _ = workflow.GetRandomStream(ctx, "order-child-id").Read(randBytes[:])
			orderWfID := fmt.Sprintf("order-%x", randBytes)

			childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: orderWfID,
				TaskQueue:  order.TaskQueue,
			})
			childFuture := workflow.ExecuteChildWorkflow(childCtx, order.OrderWorkflow, order.OrderInput{
				Customer: customer,
				Address:  "123 Tech Campus Way",
				Items:    orderItems,
			})
			// Wait for child workflow to start
			var childExec workflow.Execution
			_ = childFuture.GetChildWorkflowExecution().Get(ctx, &childExec)

			publishChat("assistant", fmt.Sprintf("🚚 Your order is live! (%s — $%.2f)", strings.Join(summaryParts, ", "), total))

			trackSID := fmt.Sprintf("track-%s", orderWfID)
			_ = ui.Publish(a2ui.CreateSurface(trackSID, false))
			_ = ui.Publish(a2ui.UpdateComponents(trackSID, []map[string]any{
				{"id": "root", "component": "Card", "child": "track_col"},
				{"id": "track_col", "component": "Column", "children": []string{"track_icon", "track_title", "track_btn"}},
				{"id": "track_icon", "component": "Text", "text": "🚚", "textStyle": map[string]any{"fontSize": 36}},
				{"id": "track_title", "component": "Text", "text": "Order is Live!", "textStyle": map[string]any{"fontWeight": "bold", "fontSize": 18}},
				{
					"id":        "track_btn",
					"component": "Button",
					"child":     "track_btn_label",
					"action":    a2ui.EventAction(fmt.Sprintf("track_order:%s", orderWfID), nil),
				},
				{"id": "track_btn_label", "component": "Text", "text": "📦 Track Your Order →"},
			}))

			orderUpdates = []string{
				"📋 Order confirmed! Payment processed.",
				"👨‍🍳 Your food is being prepared...",
				"🚚 Out for delivery! ETA 15 min.",
				fmt.Sprintf("✅ Delivered! Enjoy your meal, %s! 🎉", customer),
			}
		}

		// Stream compaction via googleadk.ExportSession + stream.NewContinueAsNewError
		if messageCounter >= MaxMessagesBeforeCompaction || workflow.GetInfo(ctx).GetContinueAsNewSuggested() {
			snap, _ := googleadk.ExportSession(adkCtx, svc, "durable-eats", customer, sessionID)
			publishChat("system", "🔄 *Session compacted — conversation history & stream state preserved via Continue-As-New.*")
			return "", stream.NewContinueAsNewError(ctx, ConciergeWorkflow, func(state *workflowstreams.WorkflowStreamState) []any {
				return []any{ConciergeInput{
					CustomerName:           customer,
					StreamState:            state,
					Cart:                   cart,
					PendingMessages:        pendingMessages,
					OrderStarted:           orderStarted,
					ConfirmedItems:         confirmedItems,
					DietaryChecked:         dietaryChecked,
					MessageCounter:         messageCounter,
					TotalTokens:            totalTokens,
					TotalCostUSD:           totalCostUSD,
					TurnCount:              turnCount,
					ToolCallCount:          toolCallCount,
					SessionSnapshot:        snap,
					ApprovalRequired:       approvalRequired,
					ApprovalDecision:       approvalDecision,
					ApprovalThreshold:      approvalThreshold,
					ApprovalTimeoutSeconds: int(approvalTimeout.Seconds()),
				}}
			})
		}
	}

	return "session_ended", nil
}
