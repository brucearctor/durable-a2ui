package incident

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

// RegisterRoutes mounts the HTTP API endpoints for Scenario 4 (CloudGuard AI SRE Incident Commander).
func RegisterRoutes(mux *http.ServeMux, tc client.Client, streamSSE a2ui.TopicSSEHandler) {
	mux.HandleFunc("POST /api/incidents", func(w http.ResponseWriter, r *http.Request) {
		var body IncidentInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		body.ApplyDefaults()
		wfID := fmt.Sprintf("incident-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: TaskQueue,
		}, IncidentWorkflow, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"workflow_id": wfID})
	})

	mux.HandleFunc("GET /api/incidents/{workflow_id}/stream", func(w http.ResponseWriter, r *http.Request) {
		streamSSE(w, r, r.PathValue("workflow_id"), "a2ui")
	})

	mux.HandleFunc("POST /api/incidents/{workflow_id}/message", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("POST /api/incidents/{workflow_id}/approve", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		var body struct {
			Decision string `json:"decision"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Decision != "approved" && body.Decision != "rejected" {
			http.Error(w, "Invalid decision", http.StatusBadRequest)
			return
		}
		if err := tc.SignalWorkflow(r.Context(), wfID, "", "commander_decision", body.Decision); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true, "decision": body.Decision})
	})

	mux.HandleFunc("POST /api/incidents/{workflow_id}/signal/{signal_name}", func(w http.ResponseWriter, r *http.Request) {
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
