package approval

import "go.temporal.io/sdk/contrib/workflowstreams"

const TaskQueue = "approval-workflow"

type ExpenseItem struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
}

type ApprovalInput struct {
	RequestID     string                       `json:"request_id"`
	Requester     string                       `json:"requester"`
	Title         string                       `json:"title"`
	Justification string                       `json:"justification"`
	Items         []ExpenseItem                `json:"items"`
	Scenario      string                       `json:"scenario,omitempty"` // "campus_hackathon" or "devfest_cloud"
	StreamState   *workflowstreams.WorkflowStreamState `json:"stream_state,omitempty"`
}

type ApprovalState struct {
	RequestID     string        `json:"request_id"`
	Requester     string        `json:"requester"`
	Title         string        `json:"title"`
	Justification string        `json:"justification"`
	Items         []ExpenseItem `json:"items"`
	Total         float64       `json:"total"`
	Status        string        `json:"status"` // submitted, under_review, approved, rejected
	DecisionNote  string        `json:"decision_note"`
}
