package council

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

// RegisterRoutes mounts the HTTP API endpoints for Scenario 5 (Parallel A2A Council & Live Schema Migration).
func RegisterRoutes(mux *http.ServeMux, tc client.Client, streamSSE a2ui.TopicSSEHandler) {
	mux.HandleFunc("POST /api/council", func(w http.ResponseWriter, r *http.Request) {
		var body CouncilInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		body.ApplyDefaults()

		wfID := fmt.Sprintf("council-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: TaskQueue,
		}, CouncilWorkflow, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"workflow_id": wfID, "target_schema": body.TargetSchema})
	})

	mux.HandleFunc("GET /api/council/{workflow_id}/stream", func(w http.ResponseWriter, r *http.Request) {
		streamSSE(w, r, r.PathValue("workflow_id"), "a2ui")
	})

	mux.HandleFunc("POST /api/council/{workflow_id}/upgrade", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		var body struct {
			Target        int `json:"target"`
			TargetVersion int `json:"target_version"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		target := body.Target
		if target == 0 {
			target = body.TargetVersion
		}
		if target == 0 {
			target = 2
		}
		if err := tc.SignalWorkflow(r.Context(), wfID, "", "upgrade_schema", target); err != nil {
			errLower := strings.ToLower(err.Error())
			if strings.Contains(errLower, "not found") || strings.Contains(errLower, "completed") {
				a2ui.WriteJSON(w, map[string]any{"ok": true, "completed": true})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true, "target": target})
	})

	decideHandler := func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		var body struct {
			Decision string `json:"decision"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Decision == "" {
			body.Decision = "approved"
		}
		if body.Decision == "rejected" {
			body.Decision = "changes_requested"
		}
		if err := tc.SignalWorkflow(r.Context(), wfID, "", "council_decision", body.Decision); err != nil {
			errLower := strings.ToLower(err.Error())
			if strings.Contains(errLower, "not found") || strings.Contains(errLower, "completed") {
				a2ui.WriteJSON(w, map[string]any{"ok": true, "completed": true})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true, "decision": body.Decision})
	}
	mux.HandleFunc("POST /api/council/{workflow_id}/decision", decideHandler)
	mux.HandleFunc("POST /api/council/{workflow_id}/decide", decideHandler)

	mux.HandleFunc("POST /api/council/{workflow_id}/signal/{signal_name}", func(w http.ResponseWriter, r *http.Request) {
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
