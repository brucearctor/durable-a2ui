package main

import (
	"os"
	"os/exec"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/googleadk"
	"go.temporal.io/sdk/worker"

	"github.com/ab/durable-a2ui/backend/pkg/approval"
	"github.com/ab/durable-a2ui/backend/pkg/concierge"
	"github.com/ab/durable-a2ui/backend/pkg/council"
	"github.com/ab/durable-a2ui/backend/pkg/incident"
	"github.com/ab/durable-a2ui/backend/pkg/order"
)

// WorkerManager manages the Temporal worker lifecycle (OS subprocess or in-process fallback).
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

	// Prefer compiled worker subprocess so SIGKILL is a genuine OS process crash
	var cmd *exec.Cmd
	if info, err := os.Stat("./bin/worker"); err == nil && !info.IsDir() {
		cmd = exec.Command("./bin/worker")
	} else if info, err := os.Stat("./backend/bin/worker"); err == nil && !info.IsDir() {
		cmd = exec.Command("./backend/bin/worker")
	}
	if cmd != nil {
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

	// Fallback: run the 5 Temporal workers in-process
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

	councilW := worker.New(c, council.TaskQueue, worker.Options{})
	councilW.RegisterWorkflow(council.CouncilWorkflow)
	councilW.RegisterActivity(&council.Activities{})

	ws := []worker.Worker{orderW, approvalW, conciergeW, incidentW, councilW}
	for _, w := range ws {
		if err := w.Start(); err != nil {
			return nil, err
		}
	}
	return ws, nil
}
