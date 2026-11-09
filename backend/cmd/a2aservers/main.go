package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ab/durable-a2ui/backend/pkg/a2aservers"
)

func main() {
	servers := []struct {
		name string
		port int
		mux  *http.ServeMux
	}{
		{
			name: "Restaurant & Campus Spot Finder A2A Agent",
			port: a2aservers.RestaurantFinderPort,
			mux:  a2aservers.RestaurantFinderServer(a2aservers.RestaurantFinderPort),
		},
		{
			name: "Dietary & Allergy Safety Advisor A2A Agent",
			port: a2aservers.DietaryAdvisorPort,
			mux:  a2aservers.DietaryAdvisorServer(a2aservers.DietaryAdvisorPort),
		},
		{
			name: "CloudGuard Telemetry & Canary Diagnostics A2A Agent",
			port: a2aservers.TelemetryDiagPort,
			mux:  a2aservers.TelemetryDiagnosticsServer(a2aservers.TelemetryDiagPort),
		},
		{
			name: "SecOps Blast-Radius & Change-Freeze Policy A2A Agent",
			port: a2aservers.SecOpsPolicyPort,
			mux:  a2aservers.SecOpsPolicyServer(a2aservers.SecOpsPolicyPort),
		},
	}

	for _, s := range servers {
		srv := s
		go func() {
			addr := fmt.Sprintf("127.0.0.1:%d", srv.port)
			log.Printf("🔗 Starting %s on http://%s (AgentCard: /.well-known/agent-card.json)", srv.name, addr)
			httpSrv := &http.Server{
				Addr:              addr,
				Handler:           srv.mux,
				ReadHeaderTimeout: 10 * time.Second,
			}
			if err := httpSrv.ListenAndServe(); err != nil {
				log.Printf("A2A server on :%d stopped: %v", srv.port, err)
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down A2A servers...")
}
