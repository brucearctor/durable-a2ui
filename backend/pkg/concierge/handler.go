package concierge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

// RegisterRoutes mounts the HTTP API endpoints for Scenario 3 (Durable Eats AI Concierge).
func RegisterRoutes(mux *http.ServeMux, tc client.Client, streamSSE a2ui.TopicSSEHandler) {
	mux.HandleFunc("POST /api/concierge", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CustomerName string `json:"customer_name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.TrimSpace(body.CustomerName) == "" {
			body.CustomerName = "Customer"
		}
		wfID := fmt.Sprintf("concierge-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: TaskQueue,
		}, ConciergeWorkflow, ConciergeInput{
			CustomerName: body.CustomerName,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"workflow_id": wfID})
	})

	mux.HandleFunc("GET /api/concierge/{workflow_id}/stream", func(w http.ResponseWriter, r *http.Request) {
		streamSSE(w, r, r.PathValue("workflow_id"), "a2ui")
	})

	mux.HandleFunc("POST /api/concierge/{workflow_id}/message", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		var body struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.TrimSpace(body.Text) == "" {
			http.Error(w, "Empty message", http.StatusBadRequest)
			return
		}
		if err := tc.SignalWorkflow(r.Context(), wfID, "", "user_message", body.Text); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("POST /api/concierge/{workflow_id}/approve", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		var body struct {
			Decision string `json:"decision"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Decision != "approved" && body.Decision != "rejected" {
			http.Error(w, "Invalid decision", http.StatusBadRequest)
			return
		}
		if err := tc.SignalWorkflow(r.Context(), wfID, "", "manager_decision", body.Decision); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true, "decision": body.Decision})
	})

	mux.HandleFunc("POST /api/concierge/{workflow_id}/signal/{signal_name}", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		sig := r.PathValue("signal_name")
		if sig == "user_action" {
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if err := tc.SignalWorkflow(r.Context(), wfID, "", sig, payload); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if err := tc.SignalWorkflow(r.Context(), wfID, "", sig, nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true})
	})
}
