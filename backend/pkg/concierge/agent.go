package concierge

import (
	"context"
	"fmt"
	"iter"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/genai"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

const ModelName = "gemini-2.5-flash"

// Tool argument and result structs for deterministic in-workflow tools

type GetMenuArgs struct {
	Category string `json:"category,omitempty"`
}

type GetMenuResult struct {
	Restaurant string         `json:"restaurant"`
	Items      []MenuItemInfo `json:"items"`
}

type AddToCartArgs struct {
	ItemName string `json:"item_name"`
	Quantity int    `json:"quantity"`
}

type AddToCartResult struct {
	Added     string  `json:"added,omitempty"`
	Quantity  int     `json:"quantity,omitempty"`
	UnitPrice float64 `json:"unit_price,omitempty"`
	Subtotal  float64 `json:"subtotal,omitempty"`
	Error     string  `json:"error,omitempty"`
}

type RemoveFromCartArgs struct {
	ItemName string `json:"item_name"`
}

type RemoveFromCartResult struct {
	Removed  string `json:"removed"`
	Quantity int    `json:"quantity"`
}

type GetCartArgs struct{}

type GetCartResult struct {
	Items     []CartItem `json:"items"`
	Total     float64    `json:"total"`
	ItemCount int        `json:"item_count"`
}

type ConfirmOrderArgs struct {
	Customer string `json:"customer"`
}

type ConfirmOrderResult struct {
	Status    string     `json:"status"`
	Reason    string     `json:"reason,omitempty"`
	Customer  string     `json:"customer,omitempty"`
	Items     []CartItem `json:"items,omitempty"`
	Total     float64    `json:"total,omitempty"`
	Threshold float64    `json:"threshold,omitempty"`
	Message   string     `json:"message"`
}

type SendA2UIArgs struct {
	A2UIJSON string `json:"a2ui_json"`
}

type SendA2UIResult struct {
	Valid   bool             `json:"valid"`
	Count   int              `json:"count,omitempty"`
	Error   string           `json:"error,omitempty"`
	A2UIMsg []map[string]any `json:"a2ui_validated_json,omitempty"`
}

// WorkflowToolHooks connects deterministic in-workflow ADK tools directly to workflow state.
type WorkflowToolHooks struct {
	OnGetMenu      func(args GetMenuArgs) (GetMenuResult, error)
	OnAddToCart    func(args AddToCartArgs) (AddToCartResult, error)
	OnRemoveCart   func(args RemoveFromCartArgs) (RemoveFromCartResult, error)
	OnGetCart      func() (GetCartResult, error)
	OnConfirmOrder func(args ConfirmOrderArgs) (ConfirmOrderResult, error)
	OnSendA2UI     func(args SendA2UIArgs) (SendA2UIResult, error)
}

func CreateConciergeAgent(hooks *WorkflowToolHooks) (agent.Agent, error) {
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 1.5,
			MaximumInterval:    3 * time.Second,
			MaximumAttempts:    20,
		},
	}

	a2aRestTool, err := googleadk.ActivityAsTool(A2AFindRestaurantsActivity, googleadk.ActivityToolOptions{
		Name:            "a2a_find_restaurants",
		Description:     "Search for local & campus restaurants via the remote A2A Restaurant Finder agent (:9001).",
		ActivityOptions: actOpts,
	})
	if err != nil {
		return nil, err
	}

	a2aDietTool, err := googleadk.ActivityAsTool(A2ACheckDietarySafetyActivity, googleadk.ActivityToolOptions{
		Name:            "a2a_check_dietary_safety",
		Description:     "Check dietary restrictions and allergens via the remote A2A Dietary Safety Advisor agent (:9002).",
		ActivityOptions: actOpts,
	})
	if err != nil {
		return nil, err
	}

	getMenuTool, err := functiontool.New(functiontool.Config{
		Name:        "get_menu",
		Description: "Get the Durable Eats menu with street tacos, campus boba, pizzas, salads, and prices.",
	}, func(_ agent.Context, args GetMenuArgs) (GetMenuResult, error) {
		return hooks.OnGetMenu(args)
	})
	if err != nil {
		return nil, err
	}

	addToCartTool, err := functiontool.New(functiontool.Config{
		Name:        "add_to_cart",
		Description: "Add a menu item and quantity to the customer's durable server-side cart.",
	}, func(_ agent.Context, args AddToCartArgs) (AddToCartResult, error) {
		return hooks.OnAddToCart(args)
	})
	if err != nil {
		return nil, err
	}

	removeCartTool, err := functiontool.New(functiontool.Config{
		Name:        "remove_from_cart",
		Description: "Remove one of an item from the customer's cart.",
	}, func(_ agent.Context, args RemoveFromCartArgs) (RemoveFromCartResult, error) {
		return hooks.OnRemoveCart(args)
	})
	if err != nil {
		return nil, err
	}

	getCartTool, err := functiontool.New(functiontool.Config{
		Name:        "get_cart",
		Description: "Get current cart items and total from durable workflow state.",
	}, func(_ agent.Context, _ GetCartArgs) (GetCartResult, error) {
		return hooks.OnGetCart()
	})
	if err != nil {
		return nil, err
	}

	confirmOrderTool, err := functiontool.New(functiontool.Config{
		Name:        "confirm_order",
		Description: "Confirm the customer's order and start the live OrderTracking child workflow.",
	}, func(_ agent.Context, args ConfirmOrderArgs) (ConfirmOrderResult, error) {
		return hooks.OnConfirmOrder(args)
	})
	if err != nil {
		return nil, err
	}

	sendA2UITool, err := functiontool.New(functiontool.Config{
		Name:        "send_a2ui_json_to_client",
		Description: "Validate and publish A2UI v1.0 JSON components to the browser surface.",
	}, func(_ agent.Context, args SendA2UIArgs) (SendA2UIResult, error) {
		if hooks.OnSendA2UI != nil {
			return hooks.OnSendA2UI(args)
		}
		msgs, err := a2ui.ParseAndValidate(args.A2UIJSON)
		if err != nil {
			return SendA2UIResult{Valid: false, Error: err.Error()}, nil
		}
		return SendA2UIResult{Valid: true, Count: len(msgs), A2UIMsg: msgs}, nil
	})
	if err != nil {
		return nil, err
	}

	m := googleadk.NewModel(ModelName)

	menuAdvisor, err := llmagent.New(llmagent.Config{
		Name:        "menu_advisor",
		Description: "Answers menu questions, recommends street tacos, campus boba, pizzas, and discovers restaurants via A2A.",
		Instruction: "AGENT_ROLE:menu_advisor\nYou are the Menu Advisor for Durable Eats. Use get_menu for menu questions and a2a_find_restaurants for restaurant recommendations.",
		Model:       m,
		Tools:       []tool.Tool{getMenuTool, a2aRestTool, sendA2UITool},
	})
	if err != nil {
		return nil, err
	}

	cartManager, err := llmagent.New(llmagent.Config{
		Name:        "cart_manager",
		Description: "Adds and removes items from the customer's cart, shows cart contents, and checks dietary safety.",
		Instruction: "AGENT_ROLE:cart_manager\nYou are the Cart Manager for Durable Eats. Use add_to_cart, remove_from_cart, get_cart, and a2a_check_dietary_safety.",
		Model:       m,
		Tools:       []tool.Tool{addToCartTool, removeCartTool, getCartTool, a2aDietTool, sendA2UITool},
	})
	if err != nil {
		return nil, err
	}

	orderConfirmer, err := llmagent.New(llmagent.Config{
		Name:        "order_confirmer",
		Description: "Verifies dietary safety via A2A and confirms the final order.",
		Instruction: "AGENT_ROLE:order_confirmer\nYou are the Order Confirmer for Durable Eats. Always ensure dietary safety is verified via a2a_check_dietary_safety and call confirm_order.",
		Model:       m,
		Tools:       []tool.Tool{confirmOrderTool, a2aDietTool, sendA2UITool},
	})
	if err != nil {
		return nil, err
	}

	root, err := llmagent.New(llmagent.Config{
		Name:        "concierge",
		Description: "Root AI food ordering concierge that routes to menu_advisor, cart_manager, and order_confirmer.",
		Instruction: "AGENT_ROLE:concierge\nYou are the AI Concierge at Durable Eats. Route menu/restaurant questions to menu_advisor, cart/dietary updates to cart_manager, and order confirmation to order_confirmer.",
		Model:       m,
		SubAgents:   []agent.Agent{menuAdvisor, cartManager, orderConfirmer},
	})
	if err != nil {
		return nil, err
	}

	return root, nil
}

// ---------------------------------------------------------------------------
// Resilient Model Factory (Live Gemini + Deterministic Multi-Agent Fallback)
// ---------------------------------------------------------------------------

func NewResilientModelFactory() googleadk.ModelFactory {
	return func(ctx context.Context, name string) (model.LLM, error) {
		fallback := &demoResilientLLM{name: name}
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
		if apiKey != "" && strings.ToLower(os.Getenv("OFFLINE_MODE")) != "true" {
			live, err := gemini.NewModel(ctx, name, nil)
			if err == nil {
				return &hybridLLM{name: name, live: live, fallback: fallback}, nil
			}
		}
		return fallback, nil
	}
}

type hybridLLM struct {
	name     string
	live     model.LLM
	fallback model.LLM
}

func (h *hybridLLM) Name() string { return h.name }

func (h *hybridLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		var emitted bool
		for resp, err := range h.live.GenerateContent(ctx, req, stream) {
			if err != nil {
				if !emitted {
					for fResp, fErr := range h.fallback.GenerateContent(ctx, req, stream) {
						if !yield(fResp, fErr) {
							return
						}
					}
				}
				return
			}
			emitted = true
			if !yield(resp, nil) {
				return
			}
		}
	}
}

// demoResilientLLM drives the exact 4-agent ADK handoff + A2A + A2UI + Guardrail + HITL pipeline
// deterministically so the conference demo works 100% offline or without API keys.
type demoResilientLLM struct {
	name string
}

func (d *demoResilientLLM) Name() string { return d.name }

func (d *demoResilientLLM) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		resp := d.decide(req)
		resp.TurnComplete = true
		resp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     180,
			CandidatesTokenCount: 95,
			TotalTokenCount:      275,
		}
		yield(resp, nil)
	}
}

func (d *demoResilientLLM) decide(req *model.LLMRequest) *model.LLMResponse {
	sys := extractSystemInstruction(req)
	lastUserText := extractLastUserText(req)
	lastFnName, lastFnResp := extractLastFunctionResponse(req)
	lowerUser := strings.ToLower(lastUserText)

	activeRole := "concierge"
	if strings.Contains(sys, "AGENT_ROLE:menu_advisor") {
		activeRole = "menu_advisor"
	} else if strings.Contains(sys, "AGENT_ROLE:cart_manager") {
		activeRole = "cart_manager"
	} else if strings.Contains(sys, "AGENT_ROLE:order_confirmer") {
		activeRole = "order_confirmer"
	}

	// If a tool (other than transfer_to_agent) just completed, decide whether to chain or respond with text.
	if lastFnName != "" && lastFnName != "transfer_to_agent" {
		switch lastFnName {
		case "get_menu":
			return textResp("Here is our **Durable Eats** menu featuring **Taquería El Sol** street tacos, **Boba & Byte Campus Lab** bubble tea, wood-fired **Bella Napoli** pizzas, and fresh salads! Click **＋ Add** on any card below or tell me what you'd like.")
		case "a2a_find_restaurants":
			return textResp("I queried our remote **Restaurant & Campus Spot Finder A2A Agent (`:9001`)** over JSON-RPC 2.0! Top picks include **Taquería El Sol** (4.9★, 15 min) for Tacos al Pastor & Gringas, **Boba & Byte Campus Lab** (4.8★, 10 min) for Brown Sugar Boba, and **Bella Napoli** (4.7★) for Margherita Pizza.")
		case "add_to_cart":
			added, _ := lastFnResp["added"].(string)
			qty := 1
			if qf, ok := lastFnResp["quantity"].(float64); ok {
				qty = int(qf)
			} else if qi, ok := lastFnResp["quantity"].(int); ok {
				qty = qi
			}
			// Check if user asked for multiple items in one message ("2 pizzas and a salad", "tacos and horchata", "boba and pizza")
			secondItem, secondQty := detectSecondItem(lowerUser, added)
			if secondItem != "" {
				return fnCallResp("add_to_cart", map[string]any{
					"item_name": secondItem,
					"quantity":  secondQty,
				})
			}
			return textResp(fmt.Sprintf("Added **%d× %s** to your durable server-side cart! I've updated your interactive Cart card below. Ready to check dietary safety or confirm your order?", qty, added))
		case "remove_from_cart":
			rem, _ := lastFnResp["removed"].(string)
			return textResp(fmt.Sprintf("Removed **%s** from your cart and refreshed your A2UI cart card.", rem))
		case "get_cart":
			return textResp("Here is your current server-side cart stored in Temporal workflow state. You can verify dietary safety via A2A or confirm your order below!")
		case "a2a_check_dietary_safety":
			if activeRole == "order_confirmer" && (strings.Contains(lowerUser, "confirm") || strings.Contains(lowerUser, "place") || strings.Contains(lowerUser, "checkout")) {
				return fnCallResp("confirm_order", map[string]any{"customer": "Customer"})
			}
			return textResp("🛡️ **A2A Dietary & Allergy Safety Check Complete (`:9002`)** — All items in your cart have been verified against our allergen database (nut-free certified). You can now safely click **✅ Confirm Order**!")
		case "confirm_order":
			status, _ := lastFnResp["status"].(string)
			switch status {
			case "blocked":
				// Programmatic guardrail fired! Automatically invoke A2A dietary check.
				return fnCallResp("a2a_check_dietary_safety", map[string]any{
					"items":        "all cart items",
					"restrictions": "general allergen & dietary safety check",
				})
			case "pending_approval":
				msg, _ := lastFnResp["message"].(string)
				return textResp(fmt.Sprintf("⏳ **Human-in-the-Loop Gate Triggered:** %s Please use the **Manager Approval Required** banner above to Approve or Reject.", msg))
			default:
				msg, _ := lastFnResp["message"].(string)
				if msg == "" {
					msg = "Order confirmed! Your live OrderTracking child workflow has been launched."
				}
				return textResp(fmt.Sprintf("🎉 **%s** Click **📦 Track Your Order →** on the card below to watch real-time kitchen & delivery updates!", msg))
			}
		}
	}

	// Determine target intent from user's message
	intent := classifyConciergeIntent(lowerUser)

	// If currently in root concierge or wrong specialist, transfer to the right specialist first!
	if intent == "menu" && activeRole != "menu_advisor" {
		return fnCallResp("transfer_to_agent", map[string]any{"agent_name": "menu_advisor"})
	}
	if intent == "restaurants" && activeRole != "menu_advisor" {
		return fnCallResp("transfer_to_agent", map[string]any{"agent_name": "menu_advisor"})
	}
	if (intent == "add_cart" || intent == "remove_cart" || intent == "get_cart" || intent == "dietary") && activeRole != "cart_manager" {
		return fnCallResp("transfer_to_agent", map[string]any{"agent_name": "cart_manager"})
	}
	if intent == "confirm" && activeRole != "order_confirmer" {
		return fnCallResp("transfer_to_agent", map[string]any{"agent_name": "order_confirmer"})
	}

	// Now we are inside the target specialist agent — invoke its tool!
	switch intent {
	case "restaurants":
		return fnCallResp("a2a_find_restaurants", map[string]any{"query": lastUserText})
	case "menu":
		return fnCallResp("get_menu", map[string]any{"category": "all"})
	case "add_cart":
		item, qty := parseItemAndQuantity(lowerUser)
		return fnCallResp("add_to_cart", map[string]any{
			"item_name": item,
			"quantity":  qty,
		})
	case "remove_cart":
		item, _ := parseItemAndQuantity(lowerUser)
		return fnCallResp("remove_from_cart", map[string]any{"item_name": item})
	case "get_cart":
		return fnCallResp("get_cart", map[string]any{})
	case "dietary":
		return fnCallResp("a2a_check_dietary_safety", map[string]any{
			"items":        lastUserText,
			"restrictions": lastUserText,
		})
	case "confirm":
		if strings.Contains(lowerUser, "rejected") || strings.Contains(lowerUser, "auto-cancelled") {
			return textResp("Understood — the manager did not approve the high-value order, so your cart has been kept intact without charging you. Let me know if you'd like to adjust quantities!")
		}
		return fnCallResp("confirm_order", map[string]any{"customer": "Customer"})
	default:
		if strings.Contains(lowerUser, "status") {
			return textResp("Your order is actively tracked by a durable Temporal Child Workflow! Click **📦 Track Your Order →** on the card above or open Tab 1 (Live Order) to inspect its state.")
		}
		return textResp("Welcome to **Durable Eats**! You can ask me to **show the menu**, **recommend taco spots or campus boba via A2A**, **add Tacos al Pastor, Boba, or Pizza to your cart**, **check dietary safety**, or **confirm your order**.")
	}
}

func classifyConciergeIntent(lower string) string {
	if strings.Contains(lower, "manager rejected") || strings.Contains(lower, "auto-cancelled") {
		return "confirm"
	}
	if strings.Contains(lower, "confirm") || strings.Contains(lower, "checkout") || strings.Contains(lower, "place my order") || strings.Contains(lower, "manager approved") {
		return "confirm"
	}
	if strings.Contains(lower, "dietary") || strings.Contains(lower, "allerg") || strings.Contains(lower, "vegan") || strings.Contains(lower, "gluten") || strings.Contains(lower, "safe") {
		return "dietary"
	}
	if strings.Contains(lower, "remove") || strings.Contains(lower, "delete") {
		return "remove_cart"
	}
	if strings.Contains(lower, "what's in my cart") || strings.Contains(lower, "whats in my cart") || strings.Contains(lower, "show cart") || strings.Contains(lower, "view cart") || lower == "cart" {
		return "get_cart"
	}
	if strings.Contains(lower, "restaurant") || strings.Contains(lower, "recommend") || strings.Contains(lower, "where") || strings.Contains(lower, "spot") {
		return "restaurants"
	}
	if strings.Contains(lower, "menu") || strings.Contains(lower, "what do you have") || strings.Contains(lower, "dessert") || strings.Contains(lower, "options") {
		return "menu"
	}
	if strings.Contains(lower, "add") || strings.Contains(lower, "order") || strings.Contains(lower, "taco") || strings.Contains(lower, "pastor") || strings.Contains(lower, "gringa") || strings.Contains(lower, "horchata") || strings.Contains(lower, "boba") || strings.Contains(lower, "pizza") || strings.Contains(lower, "margherita") || strings.Contains(lower, "pepperoni") || strings.Contains(lower, "salad") || strings.Contains(lower, "tiramisu") || strings.Contains(lower, "water") {
		return "add_cart"
	}
	return "general"
}

var qtyRegex = regexp.MustCompile(`\b(\d+)\b`)

func parseItemAndQuantity(lower string) (string, int) {
	qty := 1
	if m := qtyRegex.FindStringSubmatch(lower); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n >= 1 && n <= 99 {
			qty = n
		}
	}
	if strings.Contains(lower, "for 4 people") || strings.Contains(lower, "team") || strings.Contains(lower, "feast") {
		if qty < 10 && strings.Contains(lower, "feast") {
			qty = 10 // triggers > $100 Manager HITL Approval Gate!
		} else if qty == 1 {
			qty = 4
		}
	}
	if item, ok := FindMenuItem(lower); ok {
		return item.Name, qty
	}
	return "Tacos al Pastor", qty
}

func detectSecondItem(lower string, firstAdded string) (string, int) {
	candidates := []struct {
		kw   string
		name string
		qty  int
	}{
		{"horchata", "Horchata Artesanal", 2},
		{"gringa", "Gringa de Pastor", 2},
		{"salad", "Caesar Salad", 1},
		{"pizza", "Margherita Pizza", 2},
		{"boba", "Brown Sugar Boba", 2},
		{"tiramisu", "Tiramisu", 1},
	}
	for _, c := range candidates {
		if strings.Contains(lower, c.kw) && !strings.EqualFold(firstAdded, c.name) {
			return c.name, c.qty
		}
	}
	return "", 0
}

func extractSystemInstruction(req *model.LLMRequest) string {
	if req == nil || req.Config == nil || req.Config.SystemInstruction == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range req.Config.SystemInstruction.Parts {
		if p != nil && p.Text != "" {
			sb.WriteString(p.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func extractLastUserText(req *model.LLMRequest) string {
	if req == nil {
		return ""
	}
	for i := len(req.Contents) - 1; i >= 0; i-- {
		c := req.Contents[i]
		if c == nil || c.Role != genai.RoleUser || isSyntheticContext(c) {
			continue
		}
		for _, p := range c.Parts {
			if p != nil && p.Text != "" {
				return p.Text
			}
		}
	}
	return ""
}

func isSyntheticContext(c *genai.Content) bool {
	for _, p := range c.Parts {
		if p != nil && strings.HasPrefix(strings.TrimSpace(p.Text), "For context:") {
			return true
		}
	}
	return false
}

func extractLastFunctionResponse(req *model.LLMRequest) (string, map[string]any) {
	if req == nil || len(req.Contents) == 0 {
		return "", nil
	}
	last := req.Contents[len(req.Contents)-1]
	if last == nil {
		return "", nil
	}
	for i := len(last.Parts) - 1; i >= 0; i-- {
		p := last.Parts[i]
		if p != nil && p.FunctionResponse != nil {
			return p.FunctionResponse.Name, p.FunctionResponse.Response
		}
	}
	return "", nil
}

func fnCallResp(name string, args map[string]any) *model.LLMResponse {
	return &model.LLMResponse{
		Content: &genai.Content{
			Role: genai.RoleModel,
			Parts: []*genai.Part{
				{
					FunctionCall: &genai.FunctionCall{
						ID:   "call-" + name,
						Name: name,
						Args: args,
					},
				},
			},
		},
	}
}

func textResp(text string) *model.LLMResponse {
	return &model.LLMResponse{
		Content: genai.NewContentFromText(text, genai.RoleModel),
	}
}
