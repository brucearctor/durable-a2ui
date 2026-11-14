package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/contrib/workflowstreams"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"

	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
	"github.com/ab/durable-a2ui/backend/pkg/approval"
	"github.com/ab/durable-a2ui/backend/pkg/concierge"
	"github.com/ab/durable-a2ui/backend/pkg/incident"
	"github.com/ab/durable-a2ui/backend/pkg/order"
)

// ---------------------------------------------------------------------------
// WorkerManager — manages Temporal worker lifecycle (OS process or in-process)
// ---------------------------------------------------------------------------

type WorkerManager struct {
	mu          sync.Mutex
	tc          client.Client
	cmd         *exec.Cmd
	inProc      []worker.Worker
	alive       bool
	pid         int
	onKillHooks []func()
}

func NewWorkerManager(tc client.Client) *WorkerManager {
	return &WorkerManager{tc: tc}
}

func (wm *WorkerManager) OnKill(fn func()) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.onKillHooks = append(wm.onKillHooks, fn)
}

func (wm *WorkerManager) Start() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.alive {
		return nil
	}

	// Prefer compiled ./bin/worker subprocess so SIGKILL is a genuine process crash
	if info, err := os.Stat("./bin/worker"); err == nil && !info.IsDir() {
		cmd := exec.Command("./bin/worker")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err == nil {
			wm.cmd = cmd
			wm.alive = true
			wm.pid = cmd.Process.Pid
			go func(c *exec.Cmd) {
				_ = c.Wait()
				wm.mu.Lock()
				if wm.cmd == c {
					wm.alive = false
					wm.pid = 0
				}
				wm.mu.Unlock()
			}(cmd)
			return nil
		}
	}

	// Fallback: run the 4 Temporal workers in-process
	workers, err := startInProcWorkers(wm.tc)
	if err != nil {
		return err
	}
	wm.inProc = workers
	wm.alive = true
	wm.pid = os.Getpid()
	return nil
}

func (wm *WorkerManager) Kill() (int, error) {
	wm.mu.Lock()
	oldPID := wm.pid
	cmd := wm.cmd
	inProc := wm.inProc
	wm.cmd = nil
	wm.inProc = nil
	wm.alive = false
	wm.pid = 0
	hooks := append([]func(){}, wm.onKillHooks...)
	wm.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	for _, w := range inProc {
		w.Stop()
	}
	for _, fn := range hooks {
		fn()
	}
	return oldPID, nil
}

func (wm *WorkerManager) Restart() (int, error) {
	_, _ = wm.Kill()
	if err := wm.Start(); err != nil {
		return 0, err
	}
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return wm.pid, nil
}

func (wm *WorkerManager) Status() (bool, int) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return wm.alive, wm.pid
}

func startInProcWorkers(c client.Client) ([]worker.Worker, error) {
	worker.SetStickyWorkflowCacheSize(0)
	orderW := worker.New(c, order.TaskQueue, worker.Options{})
	orderW.RegisterWorkflow(order.OrderWorkflow)
	orderW.RegisterActivity(&order.Activities{})

	approvalW := worker.New(c, approval.TaskQueue, worker.Options{})
	approvalW.RegisterWorkflow(approval.ApprovalWorkflow)
	approvalW.RegisterActivity(&approval.Activities{})

	conciergePlugin, err := googleadk.NewPlugin(googleadk.Config{
		Models: map[string]googleadk.ModelFactory{
			concierge.ModelName: concierge.NewResilientModelFactory(),
		},
	})
	if err != nil {
		return nil, err
	}
	conciergeW := worker.New(c, concierge.TaskQueue, worker.Options{
		Plugins: []worker.Plugin{conciergePlugin},
	})
	conciergeW.RegisterWorkflow(concierge.ConciergeWorkflow)
	conciergeW.RegisterActivityWithOptions(concierge.A2AFindRestaurantsActivity, activity.RegisterOptions{Name: "a2a_find_restaurants"})
	conciergeW.RegisterActivityWithOptions(concierge.A2ACheckDietarySafetyActivity, activity.RegisterOptions{Name: "a2a_check_dietary_safety"})

	incidentPlugin, err := googleadk.NewPlugin(googleadk.Config{
		Models: map[string]googleadk.ModelFactory{
			incident.ModelName: incident.NewResilientSREModelFactory(),
		},
	})
	if err != nil {
		return nil, err
	}
	incidentW := worker.New(c, incident.TaskQueue, worker.Options{
		Plugins: []worker.Plugin{incidentPlugin},
	})
	incidentW.RegisterWorkflow(incident.IncidentWorkflow)
	incidentW.RegisterActivityWithOptions(incident.A2ADiagnoseTelemetryActivity, activity.RegisterOptions{Name: "a2a_diagnose_telemetry"})
	incidentW.RegisterActivityWithOptions(incident.A2AEvaluateBlastRadiusActivity, activity.RegisterOptions{Name: "a2a_evaluate_blast_radius"})

	ws := []worker.Worker{orderW, approvalW, conciergeW, incidentW}
	for _, w := range ws {
		if err := w.Start(); err != nil {
			return nil, err
		}
	}
	return ws, nil
}

// ---------------------------------------------------------------------------
// Fragile In-Memory Session (for Tab 1 Fragile vs. Durable comparison)
// ---------------------------------------------------------------------------

type FragileSession struct {
	mu        sync.Mutex
	id        string
	events    []string
	done      bool
	crashed   bool
	notifyCh  chan struct{}
	rushCh    chan struct{}
	ffCh      chan struct{}
	cancelCh  chan struct{}
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

// ---------------------------------------------------------------------------
// SSE Subscriber Counter
// ---------------------------------------------------------------------------

type SubscriberTracker struct {
	mu      sync.Mutex
	clients map[string]map[chan int]struct{}
}

func NewSubscriberTracker() *SubscriberTracker {
	return &SubscriberTracker{
		clients: make(map[string]map[chan int]struct{}),
	}
}

func (st *SubscriberTracker) Join(workflowID string) (chan int, int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	ch := make(chan int, 8)
	if st.clients[workflowID] == nil {
		st.clients[workflowID] = make(map[chan int]struct{})
	}
	st.clients[workflowID][ch] = struct{}{}
	count := len(st.clients[workflowID])
	for c := range st.clients[workflowID] {
		select {
		case c <- count:
		default:
		}
	}
	return ch, count
}

func (st *SubscriberTracker) Leave(workflowID string, ch chan int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if m, ok := st.clients[workflowID]; ok {
		delete(m, ch)
		count := len(m)
		if count == 0 {
			delete(st.clients, workflowID)
		} else {
			for c := range m {
				select {
				case c <- count:
				default:
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Main HTTP/SSE Server
// ---------------------------------------------------------------------------

func ensureA2AServersRunning() {
	ports := []struct {
		port int
		mux  *http.ServeMux
	}{
		{a2aservers.RestaurantFinderPort, a2aservers.RestaurantFinderServer(a2aservers.RestaurantFinderPort)},
		{a2aservers.DietaryAdvisorPort, a2aservers.DietaryAdvisorServer(a2aservers.DietaryAdvisorPort)},
		{a2aservers.TelemetryDiagPort, a2aservers.TelemetryDiagnosticsServer(a2aservers.TelemetryDiagPort)},
		{a2aservers.SecOpsPolicyPort, a2aservers.SecOpsPolicyServer(a2aservers.SecOpsPolicyPort)},
	}
	for _, p := range ports {
		addr := fmt.Sprintf("127.0.0.1:%d", p.port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue // Already running in standalone process
		}
		mux := p.mux
		port := p.port
		go func() {
			log.Printf("🔗 Embedded A2A server listening on http://127.0.0.1:%d", port)
			httpSrv := &http.Server{
				Handler:           mux,
				ReadHeaderTimeout: 10 * time.Second,
			}
			_ = httpSrv.Serve(ln)
		}()
	}
}

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "127.0.0.1:7233"
	}

	tc, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal at %s: %v", hostPort, err)
	}
	defer tc.Close()

	ensureA2AServersRunning()

	wm := NewWorkerManager(tc)
	if err := wm.Start(); err != nil {
		log.Printf("⚠️ Could not auto-start worker: %v", err)
	}

	var fragileMu sync.Mutex
	fragileSessions := make(map[string]*FragileSession)

	wm.OnKill(func() {
		fragileMu.Lock()
		defer fragileMu.Unlock()
		for _, fs := range fragileSessions {
			fs.crash()
		}
		// Wipe in-memory fragile sessions on crash!
		fragileSessions = make(map[string]*FragileSession)
	})

	subTracker := NewSubscriberTracker()

	streamTopicSSE := func(w http.ResponseWriter, r *http.Request, workflowID string, topic string) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		lastEventID := -1
		if q := r.URL.Query().Get("last_event_id"); q != "" {
			if v, err := strconv.Atoi(q); err == nil {
				lastEventID = v
			}
		} else if h := r.Header.Get("Last-Event-ID"); h != "" {
			if v, err := strconv.Atoi(h); err == nil {
				lastEventID = v
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		countCh, initialCount := subTracker.Join(workflowID)
		defer subTracker.Leave(workflowID, countCh)

		_, _ = fmt.Fprintf(w, "event: clients\ndata: {\"count\": %d}\n\n", initialCount)
		flusher.Flush()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		sc := workflowstreams.NewClient(tc, workflowID, workflowstreams.Options{})
		dc := converter.GetDefaultDataConverter()
		var writeMu sync.Mutex

		go func() {
			keepalive := time.NewTicker(15 * time.Second)
			defer keepalive.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case c := <-countCh:
					writeMu.Lock()
					_, _ = fmt.Fprintf(w, "event: clients\ndata: {\"count\": %d}\n\n", c)
					flusher.Flush()
					writeMu.Unlock()
				case <-keepalive.C:
					writeMu.Lock()
					_, _ = fmt.Fprintf(w, ": keepalive\n\n")
					flusher.Flush()
					writeMu.Unlock()
				}
			}
		}()

		nextOffset := int64(lastEventID + 1)
		for item, err := range sc.Subscribe(ctx, workflowstreams.SubscribeOptions{
			Topics:       []string{topic},
			FromOffset:   nextOffset,
			PollCooldown: 200 * time.Millisecond,
		}) {
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("SSE stream error for %s/%s: %v", workflowID, topic, err)
				}
				return
			}
			if item.Data != nil {
				var val any
				if err := dc.FromPayload(item.Data, &val); err == nil {
					b, _ := json.Marshal(val)
					writeMu.Lock()
					_, _ = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", item.Offset, string(b))
					flusher.Flush()
					writeMu.Unlock()
					if item.Offset+1 > nextOffset {
						nextOffset = item.Offset + 1
					}
				}
			}
		}

		if ctx.Err() == nil {
			if qv, qerr := tc.QueryWorkflow(ctx, workflowID, "", a2ui.StreamStateQueryName); qerr == nil {
				var st workflowstreams.WorkflowStreamState
				if qv.Get(&st) == nil {
					for i, wi := range st.Log {
						globalOffset := st.BaseOffset + int64(i)
						if globalOffset < nextOffset || wi.Topic != topic {
							continue
						}
						if val, derr := a2ui.DecodeWireItem(wi.Data, dc); derr == nil {
							b, _ := json.Marshal(val)
							writeMu.Lock()
							_, _ = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", globalOffset, string(b))
							flusher.Flush()
							writeMu.Unlock()
							nextOffset = globalOffset + 1
						}
					}
				}
			}
			writeMu.Lock()
			_, _ = fmt.Fprintf(w, "event: done\ndata: {}\n\n")
			flusher.Flush()
			writeMu.Unlock()
		}
	}

	mux := http.NewServeMux()

	// ── Config & A2A Discovery ─────────────────────────────────────────
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"public_url": os.Getenv("PUBLIC_URL")})
	})

	mux.HandleFunc("GET /api/a2a/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"agents": []map[string]any{
				{"name": "Restaurant & Campus Spot Finder", "port": a2aservers.RestaurantFinderPort, "protocol": "A2A JSON-RPC 2.0"},
				{"name": "Dietary & Allergy Safety Advisor", "port": a2aservers.DietaryAdvisorPort, "protocol": "A2A JSON-RPC 2.0"},
				{"name": "CloudGuard Telemetry & Canary Diagnostics", "port": a2aservers.TelemetryDiagPort, "protocol": "A2A JSON-RPC 2.0"},
				{"name": "SecOps Blast-Radius & Change-Freeze Policy", "port": a2aservers.SecOpsPolicyPort, "protocol": "A2A JSON-RPC 2.0"},
			},
		})
	})

	// ── Worker Management (Kill / Restart / Status) ────────────────────
	mux.HandleFunc("GET /api/worker/status", func(w http.ResponseWriter, _ *http.Request) {
		alive, pid := wm.Status()
		writeJSON(w, map[string]any{"alive": alive, "pid": pid})
	})

	mux.HandleFunc("POST /api/worker/kill", func(w http.ResponseWriter, _ *http.Request) {
		pid, err := wm.Kill()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"killed": true, "pid": pid})
	})

	mux.HandleFunc("POST /api/worker/restart", func(w http.ResponseWriter, _ *http.Request) {
		pid, err := wm.Restart()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"restarted": true, "pid": pid})
	})

	// ── Tab 1: Durable Order Tracking (/api/sessions) ──────────────────
	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
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

		wfID := fmt.Sprintf("order-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: order.TaskQueue,
		}, order.OrderWorkflow, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"workflow_id": wfID})
	})

	mux.HandleFunc("GET /api/sessions/{workflow_id}/stream", func(w http.ResponseWriter, r *http.Request) {
		streamTopicSSE(w, r, r.PathValue("workflow_id"), "a2ui")
	})

	mux.HandleFunc("POST /api/sessions/{workflow_id}/signal/{signal_name}", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		sig := r.PathValue("signal_name")
		if err := tc.SignalWorkflow(r.Context(), wfID, "", sig, nil); err != nil {
			errLower := strings.ToLower(err.Error())
			if strings.Contains(errLower, "not found") || strings.Contains(errLower, "completed") {
				writeJSON(w, map[string]any{"ok": true, "completed": true})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// ── Tab 1: Fragile In-Memory Order Tracking (/api/fragile-sessions) ─
	mux.HandleFunc("POST /api/fragile-sessions", func(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, map[string]any{"workflow_id": sid})
	})

	mux.HandleFunc("GET /api/fragile-sessions/{session_id}/stream", func(w http.ResponseWriter, r *http.Request) {
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
	})

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
		writeJSON(w, map[string]any{"ok": true})
	})

	// ── Tab 2: Dual-Surface Approval (/api/approvals) ──────────────────
	mux.HandleFunc("POST /api/approvals", func(w http.ResponseWriter, r *http.Request) {
		var body approval.ApprovalInput
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
			body.Items = []approval.ExpenseItem{
				{Description: "GKE L4 GPU Inference Node Pool (Multi-Zone)", Amount: 680.00, Category: "Cloud Compute"},
				{Description: "AlloyDB Read-Replica High-Availability Pool", Amount: 720.00, Category: "Database"},
				{Description: "OpenTelemetry & Distributed Trace Retention Quota", Amount: 485.00, Category: "Observability"},
				{Description: "Student Hackathon Hardware & Swag Kit", Amount: 250.00, Category: "Materials"},
			}
		}

		wfID := fmt.Sprintf("approval-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: approval.TaskQueue,
		}, approval.ApprovalWorkflow, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"workflow_id": wfID, "request_id": body.RequestID})
	})

	mux.HandleFunc("GET /api/approvals/{workflow_id}/stream/{role}", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		role := r.PathValue("role")
		if role != "requester" && role != "approver" {
			http.Error(w, "Invalid role", http.StatusBadRequest)
			return
		}
		streamTopicSSE(w, r, wfID, role+"-ui")
	})

	mux.HandleFunc("POST /api/approvals/{workflow_id}/signal/{signal_name}", func(w http.ResponseWriter, r *http.Request) {
		wfID := r.PathValue("workflow_id")
		sig := r.PathValue("signal_name")
		var body struct {
			Note string `json:"note"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := tc.SignalWorkflow(r.Context(), wfID, "", sig, body.Note); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// ── Tab 3: AI Concierge (/api/concierge) ───────────────────────────
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
			TaskQueue: concierge.TaskQueue,
		}, concierge.ConciergeWorkflow, concierge.ConciergeInput{
			CustomerName: body.CustomerName,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"workflow_id": wfID})
	})

	mux.HandleFunc("GET /api/concierge/{workflow_id}/stream", func(w http.ResponseWriter, r *http.Request) {
		streamTopicSSE(w, r, r.PathValue("workflow_id"), "a2ui")
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
		writeJSON(w, map[string]any{"ok": true})
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
		writeJSON(w, map[string]any{"ok": true, "decision": body.Decision})
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
		writeJSON(w, map[string]any{"ok": true})
	})

	// ── Tab 4: CloudGuard AI SRE Incident Commander (/api/incidents) ───
	mux.HandleFunc("POST /api/incidents", func(w http.ResponseWriter, r *http.Request) {
		var body incident.IncidentInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		body.ApplyDefaults()
		wfID := fmt.Sprintf("incident-%s", uuid.New().String()[:8])
		_, err := tc.ExecuteWorkflow(r.Context(), client.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: incident.TaskQueue,
		}, incident.IncidentWorkflow, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"workflow_id": wfID})
	})

	mux.HandleFunc("GET /api/incidents/{workflow_id}/stream", func(w http.ResponseWriter, r *http.Request) {
		streamTopicSSE(w, r, r.PathValue("workflow_id"), "a2ui")
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
		writeJSON(w, map[string]any{"ok": true})
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
		writeJSON(w, map[string]any{"ok": true, "decision": body.Decision})
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
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /slides", func(w http.ResponseWriter, r *http.Request) {
		for _, p := range []string{"slides/index.html", "../slides/index.html"} {
			if _, err := os.Stat(p); err == nil {
				http.ServeFile(w, r, p)
				return
			}
		}
		http.Error(w, "slides/index.html not found", http.StatusNotFound)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	addr := "0.0.0.0:" + port
	log.Printf("🚀 Durable A2UI Go BFF Server listening on http://%s", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		_, _ = wm.Kill()
		_ = srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Last-Event-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
