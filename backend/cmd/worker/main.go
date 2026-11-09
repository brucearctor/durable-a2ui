package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/worker"

	"github.com/ab/durable-a2ui/backend/pkg/approval"
	"github.com/ab/durable-a2ui/backend/pkg/concierge"
	"github.com/ab/durable-a2ui/backend/pkg/incident"
	"github.com/ab/durable-a2ui/backend/pkg/order"
)

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "127.0.0.1:7233"
	}

	c, err := client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal at %s: %v", hostPort, err)
	}
	defer c.Close()

	// 1. Order Tracking Worker
	orderWorker := worker.New(c, order.TaskQueue, worker.Options{})
	orderWorker.RegisterWorkflow(order.OrderWorkflow)
	orderWorker.RegisterActivity(&order.Activities{})

	// 2. Dual-Surface Approval Worker
	approvalWorker := worker.New(c, approval.TaskQueue, worker.Options{})
	approvalWorker.RegisterWorkflow(approval.ApprovalWorkflow)
	approvalWorker.RegisterActivity(&approval.Activities{})

	// 3. AI Concierge Worker (with googleadk Plugin + A2A Activities)
	conciergePlugin, err := googleadk.NewPlugin(googleadk.Config{
		Models: map[string]googleadk.ModelFactory{
			concierge.ModelName: concierge.NewResilientModelFactory(),
		},
	})
	if err != nil {
		log.Fatalf("Failed to create googleadk plugin for concierge: %v", err)
	}
	conciergeWorker := worker.New(c, concierge.TaskQueue, worker.Options{
		Plugins: []worker.Plugin{conciergePlugin},
	})
	conciergeWorker.RegisterWorkflow(concierge.ConciergeWorkflow)
	conciergeWorker.RegisterActivityWithOptions(concierge.A2AFindRestaurantsActivity, activity.RegisterOptions{
		Name: "a2a_find_restaurants",
	})
	conciergeWorker.RegisterActivityWithOptions(concierge.A2ACheckDietarySafetyActivity, activity.RegisterOptions{
		Name: "a2a_check_dietary_safety",
	})

	// 4. CloudGuard AI SRE Incident Commander Worker (with googleadk Plugin + A2A Activities)
	incidentPlugin, err := googleadk.NewPlugin(googleadk.Config{
		Models: map[string]googleadk.ModelFactory{
			incident.ModelName: incident.NewResilientSREModelFactory(),
		},
	})
	if err != nil {
		log.Fatalf("Failed to create googleadk plugin for incident commander: %v", err)
	}
	incidentWorker := worker.New(c, incident.TaskQueue, worker.Options{
		Plugins: []worker.Plugin{incidentPlugin},
	})
	incidentWorker.RegisterWorkflow(incident.IncidentWorkflow)
	incidentWorker.RegisterActivityWithOptions(incident.A2ADiagnoseTelemetryActivity, activity.RegisterOptions{
		Name: "a2a_diagnose_telemetry",
	})
	incidentWorker.RegisterActivityWithOptions(incident.A2AEvaluateBlastRadiusActivity, activity.RegisterOptions{
		Name: "a2a_evaluate_blast_radius",
	})

	workers := []worker.Worker{orderWorker, approvalWorker, conciergeWorker, incidentWorker}
	for _, w := range workers {
		if err := w.Start(); err != nil {
			log.Fatalf("Failed to start worker: %v", err)
		}
	}

	log.Printf("✅ Temporal Go Workers listening on queues: [%s, %s, %s, %s] (PID: %d)",
		order.TaskQueue, approval.TaskQueue, concierge.TaskQueue, incident.TaskQueue, os.Getpid())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Stopping Temporal Go workers...")
	for _, w := range workers {
		w.Stop()
	}
}
