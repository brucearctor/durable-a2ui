package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
	"github.com/ab/durable-a2ui/backend/pkg/approval"
	"github.com/ab/durable-a2ui/backend/pkg/concierge"
	"github.com/ab/durable-a2ui/backend/pkg/council"
	"github.com/ab/durable-a2ui/backend/pkg/incident"
	"github.com/ab/durable-a2ui/backend/pkg/order"
)

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

	subTracker := NewSubscriberTracker()
	streamTopicSSE := NewTopicSSEHandler(tc, subTracker)

	mux := http.NewServeMux()

	// ── Health, Config, A2A Discovery & Chaos Outage Simulation ────────
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		alive, pid := wm.Status()
		a2ui.WriteJSON(w, map[string]any{
			"status":  "ok",
			"runtime": "go",
			"worker":  alive,
			"pid":     pid,
		})
	})

	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, _ *http.Request) {
		a2ui.WriteJSON(w, map[string]any{"public_url": os.Getenv("PUBLIC_URL")})
	})

	mux.HandleFunc("GET /api/a2a/status", func(w http.ResponseWriter, r *http.Request) {
		a2ui.WriteJSON(w, map[string]any{
			"outage": a2aservers.GetClusterChaos(r.Context()),
			"agents": []map[string]any{
				{"name": "Restaurant & Campus Spot Finder", "port": a2aservers.RestaurantFinderPort, "protocol": "A2A JSON-RPC 2.0"},
				{"name": "Dietary & Allergy Safety Advisor", "port": a2aservers.DietaryAdvisorPort, "protocol": "A2A JSON-RPC 2.0"},
				{"name": "CloudGuard Telemetry & Canary Diagnostics", "port": a2aservers.TelemetryDiagPort, "protocol": "A2A JSON-RPC 2.0"},
				{"name": "SecOps Blast-Radius & Change-Freeze Policy", "port": a2aservers.SecOpsPolicyPort, "protocol": "A2A JSON-RPC 2.0"},
			},
		})
	})

	mux.HandleFunc("GET /api/a2a/chaos", func(w http.ResponseWriter, r *http.Request) {
		a2ui.WriteJSON(w, map[string]any{"outage": a2aservers.GetClusterChaos(r.Context())})
	})

	mux.HandleFunc("POST /api/a2a/chaos", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Outage bool `json:"outage"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		a2aservers.SetClusterChaos(r.Context(), body.Outage)
		a2ui.WriteJSON(w, map[string]any{"outage": a2aservers.GetClusterChaos(r.Context())})
	})

	// ── Worker Management (Kill / Restart / Status) ────────────────────
	workerStatusHandler := func(w http.ResponseWriter, _ *http.Request) {
		alive, pid := wm.Status()
		a2ui.WriteJSON(w, map[string]any{"running": alive, "alive": alive, "pid": pid})
	}
	mux.HandleFunc("GET /api/worker/status", workerStatusHandler)
	mux.HandleFunc("GET /api/admin/worker-status", workerStatusHandler)

	killWorkerHandler := func(w http.ResponseWriter, _ *http.Request) {
		pid, err := wm.Kill()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"killed": true, "pid": pid})
	}
	mux.HandleFunc("POST /api/worker/kill", killWorkerHandler)
	mux.HandleFunc("POST /api/admin/kill-worker", killWorkerHandler)

	restartWorkerHandler := func(w http.ResponseWriter, _ *http.Request) {
		pid, err := wm.Restart()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a2ui.WriteJSON(w, map[string]any{"restarted": true, "pid": pid})
	}
	mux.HandleFunc("POST /api/worker/restart", restartWorkerHandler)
	mux.HandleFunc("POST /api/admin/start-worker", restartWorkerHandler)

	// ── Self-Contained Scenario Modules ────────────────────────────────
	// Tab 1: Fragile vs. Durable Order Tracking
	order.RegisterRoutes(mux, tc, streamTopicSSE)
	RegisterFragileRoutes(mux, wm)

	// Tab 2: Dual-Surface Approval & Room Quorum
	approval.RegisterRoutes(mux, tc, streamTopicSSE)

	// Tab 3: Durable Eats AI Concierge (ADK + A2A + Guardrail + HITL)
	concierge.RegisterRoutes(mux, tc, streamTopicSSE)

	// Tab 4: CloudGuard AI SRE Incident Commander (ADK + A2A + SecOps + HITL)
	incident.RegisterRoutes(mux, tc, streamTopicSSE)

	// Tab 5: Parallel A2A Council & Live Schema Migration (workflow.Go + workflow.GetVersion)
	council.RegisterRoutes(mux, tc, streamTopicSSE)

	// ── Slide Deck ─────────────────────────────────────────────────────
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
