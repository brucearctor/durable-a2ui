package order

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

// RegisterRoutes mounts the HTTP API endpoints for Scenario 1 (Durable Order Tracking).
func RegisterRoutes(mux *http.ServeMux, tc client.Client, streamSSE a2ui.TopicSSEHandler) {
	createOrder := func(w http.ResponseWriter, r *http.Request) {
		var body OrderInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Customer == "" {
			body.Customer = "Alice"
		}
		if body.Address == "" {
			body.Address = "123 Main St"
		}
		if len(body.Items) == 0 {
			body.Items = []OrderItem{
				{Name: "Margherita Pizza", Qty: 1, Price: 14.99},
				{Name: "Garlic Bread", Qty: 2, Price: 4.99},
			}
		}

		wfID := fmt.Sprintf("order-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: TaskQueue,
		}, OrderWorkflow, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"workflow_id": wfID})
	}
	mux.HandleFunc("POST /api/sessions", createOrder)
	mux.HandleFunc("POST /api/orders", createOrder)

	streamOrder := func(w http.ResponseWriter, r *http.Request) {
		streamSSE(w, r, r.PathValue("workflow_id"), "a2ui")
	}
	mux.HandleFunc("GET /api/sessions/{workflow_id}/stream", streamOrder)
	mux.HandleFunc("GET /api/orders/{workflow_id}/stream", streamOrder)

	signalOrder := func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		sig := r.PathValue("signal_name")
		if err := tc.SignalWorkflow(r.Context(), wfID, "", sig, nil); err != nil {
			errLower := strings.ToLower(err.Error())
			if strings.Contains(errLower, "not found") || strings.Contains(errLower, "completed") {
				a2ui.WriteJSON(w, map[string]any{"ok": true, "completed": true})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true})
	}
	mux.HandleFunc("POST /api/sessions/{workflow_id}/signal/{signal_name}", signalOrder)
	mux.HandleFunc("POST /api/orders/{workflow_id}/signal/{signal_name}", signalOrder)
}
