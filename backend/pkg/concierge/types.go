package concierge

import (
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/contrib/workflowstreams"
)

const TaskQueue = "ai-concierge"

type CartItem struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type ConciergeInput struct {
	CustomerName           string                       `json:"customer_name"`
	StreamState            *workflowstreams.WorkflowStreamState `json:"stream_state,omitempty"`
	Cart                   map[string]CartItem          `json:"cart,omitempty"`
	PendingMessages        []string                     `json:"pending_messages,omitempty"`
	OrderStarted           bool                         `json:"order_started,omitempty"`
	ConfirmedItems         []CartItem                   `json:"confirmed_items,omitempty"`
	DietaryChecked         bool                         `json:"dietary_checked,omitempty"`
	MessageCounter         int                          `json:"message_counter,omitempty"`
	TotalTokens            int                          `json:"total_tokens,omitempty"`
	TotalCostUSD           float64                      `json:"total_cost_usd,omitempty"`
	TurnCount              int                          `json:"turn_count,omitempty"`
	ToolCallCount          int                          `json:"tool_call_count,omitempty"`
	SessionSnapshot        *googleadk.SessionSnapshot   `json:"session_snapshot,omitempty"`
	ApprovalRequired       bool                         `json:"approval_required,omitempty"`
	ApprovalDecision       string                       `json:"approval_decision,omitempty"`
	ApprovalThreshold      float64                      `json:"approval_threshold,omitempty"`
	ApprovalTimeoutSeconds int                          `json:"approval_timeout_seconds,omitempty"`
}

func (in *ConciergeInput) ApplyDefaults() {
	if in.CustomerName == "" {
		in.CustomerName = "Customer"
	}
	if in.Cart == nil {
		in.Cart = make(map[string]CartItem)
	}
	if in.ApprovalThreshold <= 0 {
		in.ApprovalThreshold = 100.0
	}
	if in.ApprovalTimeoutSeconds <= 0 {
		in.ApprovalTimeoutSeconds = 300
	}
}
