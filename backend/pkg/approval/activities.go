package approval

import (
	"context"
	"fmt"
	"time"
)

type Activities struct{}

type ValidateReq struct {
	RequestID string  `json:"request_id"`
	Total     float64 `json:"total"`
}

func (a *Activities) ValidateExpense(ctx context.Context, req ValidateReq) (string, error) {
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return fmt.Sprintf("Validated expense #%s ($%.2f) against policy", req.RequestID, req.Total), nil
}

type NotifyManagerReq struct {
	RequestID string  `json:"request_id"`
	Requester string  `json:"requester"`
	Total     float64 `json:"total"`
}

func (a *Activities) NotifyManager(ctx context.Context, req NotifyManagerReq) (string, error) {
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return fmt.Sprintf("Notified manager about #%s from %s ($%.2f)", req.RequestID, req.Requester, req.Total), nil
}

type ProcessDecisionReq struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
	Note      string `json:"note"`
}

func (a *Activities) ProcessDecision(ctx context.Context, req ProcessDecisionReq) (string, error) {
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return fmt.Sprintf("Processed %s for #%s: %s", req.Decision, req.RequestID, req.Note), nil
}
