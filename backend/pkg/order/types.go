package order

import "go.temporal.io/sdk/contrib/workflowstreams"

const TaskQueue = "order-tracking"

type OrderItem struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type OrderInput struct {
	Customer    string                       `json:"customer"`
	Address     string                       `json:"address"`
	Items       []OrderItem                  `json:"items"`
	StreamState *workflowstreams.WorkflowStreamState `json:"stream_state,omitempty"`
}

type DriverInfo struct {
	Name    string  `json:"name"`
	Vehicle string  `json:"vehicle"`
	Rating  float64 `json:"rating"`
}

type OrderState struct {
	OrderID      string      `json:"order_id"`
	Customer     string      `json:"customer"`
	Address      string      `json:"address"`
	Items        []OrderItem `json:"items"`
	Total        float64     `json:"total"`
	Status       string      `json:"status"`
	Step         int         `json:"step"`
	TotalSteps   int         `json:"total_steps"`
	ETAMinutes   int         `json:"eta_minutes"`
	Rushed       bool        `json:"rushed"`
	Cancelled    bool        `json:"cancelled"`
	CancelReason string      `json:"cancel_reason,omitempty"`
	Driver       *DriverInfo `json:"driver,omitempty"`
}
