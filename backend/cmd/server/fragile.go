package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
	"github.com/ab/durable-a2ui/backend/pkg/order"
)

// FragileSession intentionally holds order tracking state in process memory
// so students can contrast in-memory loss against Temporal durability on Tab 1.
type FragileSession struct {
	mu       sync.Mutex
	id       string
	events   []string
	done     bool
	crashed  bool
	notifyCh chan struct{}
	rushCh   chan struct{}
	ffCh     chan struct{}
	cancelCh chan struct{}
}

func newFragileSession(id string, input order.OrderInput) *FragileSession {
	fs := &FragileSession{
		id:       id,
		notifyCh: make(chan struct{}),
		rushCh:   make(chan struct{}, 4),
		ffCh:     make(chan struct{}, 4),
		cancelCh: make(chan struct{}, 4),
	}
	go fs.run(input)
	return fs
}

func (fs *FragileSession) publish(msgs []map[string]any) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.crashed || fs.done {
		return
	}
	for _, m := range msgs {
		b, _ := json.Marshal(m)
		fs.events = append(fs.events, string(b))
	}
	close(fs.notifyCh)
	fs.notifyCh = make(chan struct{})
}

func (fs *FragileSession) crash() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.done || fs.crashed {
		return
	}
	fs.crashed = true
	close(fs.notifyCh)
	fs.notifyCh = make(chan struct{})
}

func (fs *FragileSession) run(input order.OrderInput) {
	var total float64
	for _, it := range input.Items {
		total += it.Price * float64(it.Qty)
	}
	state := order.OrderState{
		OrderID:    strings.ToUpper(uuid.New().String()[:6]),
		Customer:   input.Customer,
		Address:    input.Address,
		Items:      input.Items,
		Total:      total,
		Status:     "confirmed",
		Step:       1,
		TotalSteps: 4,
		ETAMinutes: 35,
	}

	fs.publish(order.CreateOrderSurface())
	fs.publish(order.BuildInitialComponents(state))
	fs.publish(order.BuildInitialDataModel(state))

	stages := []struct {
		name string
		eta  int
	}{
		{"confirmed", 30},
		{"preparing", 20},
		{"out_for_delivery", 10},
		{"delivered", 0},
	}

	rushed := false
	for i, stg := range stages {
		select {
		case <-fs.cancelCh:
			state.Status = "cancelled"
			state.Cancelled = true
			state.CancelReason = "Cancelled by customer"
			fs.publish(order.UpdateCancelled(state))
			fs.mu.Lock()
			fs.done = true
			close(fs.notifyCh)
			fs.notifyCh = make(chan struct{})
			fs.mu.Unlock()
			return
		case <-fs.rushCh:
			if !rushed {
				rushed = true
				state.Rushed = true
			}
		case <-fs.ffCh:
		case <-time.After(3 * time.Second):
		}

		fs.mu.Lock()
		if fs.crashed {
			fs.mu.Unlock()
			return
		}
		fs.mu.Unlock()

		state.Status = stg.name
		state.Step = i + 1
		if rushed {
			state.ETAMinutes = stg.eta / 2
		} else {
			state.ETAMinutes = stg.eta
		}
		if stg.name == "out_for_delivery" && state.Driver == nil {
			state.Driver = &order.DriverInfo{Name: "Carlos M.", Vehicle: "Honda PCX 150", Rating: 4.9}
			fs.publish(order.AddDriverCard(state))
		}
		fs.publish(order.UpdateStepProgress(state))
	}

	fs.mu.Lock()
	fs.done = true
	close(fs.notifyCh)
	fs.notifyCh = make(chan struct{})
	fs.mu.Unlock()
}

func RegisterFragileRoutes(mux *http.ServeMux, wm *WorkerManager) {
	var fragileMu sync.Mutex
	fragileSessions := make(map[string]*FragileSession)

	wm.OnKill(func() {
		fragileMu.Lock()
		defer fragileMu.Unlock()
		for _, fs := range fragileSessions {
			fs.crash()
		}
		fragileSessions = make(map[string]*FragileSession)
	})

	createFragile := func(w http.ResponseWriter, r *http.Request) {
		var body order.OrderInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Customer == "" {
			body.Customer = "Alice"
		}
		if body.Address == "" {
			body.Address = "123 Main St"
		}
		if len(body.Items) == 0 {
			body.Items = []order.OrderItem{
				{Name: "Margherita Pizza", Qty: 1, Price: 14.99},
				{Name: "Garlic Bread", Qty: 2, Price: 4.99},
			}
		}
		sid := fmt.Sprintf("fragile-%s", uuid.New().String()[:8])
		fs := newFragileSession(sid, body)
		fragileMu.Lock()
		fragileSessions[sid] = fs
		fragileMu.Unlock()
		a2ui.WriteJSON(w, map[string]any{"workflow_id": sid, "order_id": sid})
	}
	mux.HandleFunc("POST /api/fragile-sessions", createFragile)
	mux.HandleFunc("POST /api/fragile/orders", createFragile)

	streamFragile := func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}
		sid := r.PathValue("session_id")
		fragileMu.Lock()
		fs := fragileSessions[sid]
		fragileMu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		if fs == nil {
			_, _ = fmt.Fprintf(w, "event: crashed\ndata: {\"error\": \"Fragile in-memory state was permanently lost when the worker crashed!\"}\n\n")
			flusher.Flush()
			return
		}

		offset := 0
		for {
			fs.mu.Lock()
			for offset < len(fs.events) {
				ev := fs.events[offset]
				_, _ = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", offset, ev)
				offset++
			}
			crashed := fs.crashed
			done := fs.done
			ch := fs.notifyCh
			fs.mu.Unlock()
			flusher.Flush()

			if crashed {
				_, _ = fmt.Fprintf(w, "event: crashed\ndata: {\"error\": \"Fragile in-memory state was permanently lost when the worker crashed!\"}\n\n")
				flusher.Flush()
				return
			}
			if done {
				_, _ = fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}

			select {
			case <-r.Context().Done():
				return
			case <-ch:
			}
		}
	}
	mux.HandleFunc("GET /api/fragile-sessions/{session_id}/stream", streamFragile)
	mux.HandleFunc("GET /api/fragile/orders/{session_id}/stream", streamFragile)

	mux.HandleFunc("POST /api/fragile-sessions/{session_id}/signal/{signal_name}", func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("session_id")
		sig := r.PathValue("signal_name")
		fragileMu.Lock()
		fs := fragileSessions[sid]
		fragileMu.Unlock()
		if fs == nil {
			http.Error(w, "Session lost", http.StatusNotFound)
			return
		}
		switch sig {
		case "rush_order":
			select {
			case fs.rushCh <- struct{}{}:
			default:
			}
		case "fast_forward":
			select {
			case fs.ffCh <- struct{}{}:
			default:
			}
		case "cancel_order":
			select {
			case fs.cancelCh <- struct{}{}:
			default:
			}
		}
		a2ui.WriteJSON(w, map[string]any{"ok": true})
	})
}
