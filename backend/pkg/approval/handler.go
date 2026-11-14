package approval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

// RegisterRoutes mounts the HTTP API endpoints for Scenario 2 (Dual-Surface Approval & Room Quorum).
func RegisterRoutes(mux *http.ServeMux, tc client.Client, streamSSE a2ui.TopicSSEHandler) {
	mux.HandleFunc("POST /api/approvals", func(w http.ResponseWriter, r *http.Request) {
		var body ApprovalInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.RequestID == "" {
			body.RequestID = fmt.Sprintf("EXP-%s", strings.ToUpper(uuid.New().String()[:4]))
		}
		if body.Requester == "" {
			body.Requester = "Sarah Chen"
		}
		if body.Title == "" {
			body.Title = "Production AI Inference — GKE L4 GPU Cluster & AlloyDB Quota Grant"
		}
		if body.Justification == "" {
			body.Justification = "Provisioning dedicated GKE L4 GPU inference nodes, AlloyDB read-replica capacity, and OpenTelemetry trace retention for multi-agent workloads."
		}
		if len(body.Items) == 0 {
			body.Items = []ExpenseItem{
				{Description: "GKE L4 GPU Inference Node Pool (Multi-Zone)", Amount: 680.00, Category: "Cloud Compute"},
				{Description: "AlloyDB Read-Replica High-Availability Pool", Amount: 720.00, Category: "Database"},
				{Description: "OpenTelemetry & Distributed Trace Retention Quota", Amount: 485.00, Category: "Observability"},
				{Description: "Student Hackathon Hardware & Swag Kit", Amount: 250.00, Category: "Materials"},
			}
		}
		if body.RequiredApprovals <= 0 {
			body.RequiredApprovals = 1
		}

		wfID := fmt.Sprintf("approval-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: TaskQueue,
		}, ApprovalWorkflow, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{
			"workflow_id":        wfID,
			"request_id":         body.RequestID,
			"required_approvals": body.RequiredApprovals,
		})
	})

	mux.HandleFunc("GET /api/approvals/{workflow_id}/stream/{role}", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		role := r.PathValue("role")
		if role != "requester" && role != "approver" {
			http.Error(w, "Invalid role", http.StatusBadRequest)
			return
		}
		streamSSE(w, r, wfID, role+"-ui")
	})

	mux.HandleFunc("POST /api/approvals/{workflow_id}/signal/{signal_name}", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		sig := r.PathValue("signal_name")
		var body struct {
			Note string `json:"note"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := tc.SignalWorkflow(r.Context(), wfID, "", sig, body.Note); err != nil {
			errLower := strings.ToLower(err.Error())
			if strings.Contains(errLower, "not found") || strings.Contains(errLower, "completed") {
				a2ui.WriteJSON(w, map[string]any{"ok": true, "completed": true})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true})
	})
}
