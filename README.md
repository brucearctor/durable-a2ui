# The Agent Interoperability Stack: ADK + A2A + A2UI — and Why Durability Changes Everything (All-Go Edition)

This repository powers the live talk and interactive demos for **The Agent Interoperability Stack: ADK + A2A + A2UI — and Why Durability Changes Everything**.

Every backend component—workflows, multi-agent orchestration, remote A2A servers, A2UI v1.0 message builders, and the HTTP/SSE BFF—is written in **100% Go**:

* **🧠 Agent Orchestration (`ADK-Go v2.7.1`)**: `google.golang.org/adk/v2` + `go.temporal.io/sdk/contrib/googleadk v0.3.0` (`googleadk.NewPlugin`, `googleadk.NewContext`, `googleadk.NewModel`, `googleadk.ActivityAsTool`, `googleadk.ExportSession` / `ImportSession`).
* **🔗 Agent-to-Agent Protocol (`A2A-Go v2.8.0`)**: `github.com/a2aproject/a2a-go/v2` (`a2asrv.NewJSONRPCHandler`, `a2asrv.NewStaticAgentCardHandler`, `agentcard.DefaultResolver`, `a2aclient.NewFromCard`) running 4 real HTTP JSON-RPC 2.0 servers on `:9001`–`:9004`.
* **🎨 Agent-to-UI Protocol (`A2UI v1.0`)**: Declarative component trees (`createSurface`, `updateComponents`, `updateDataModel`, `deleteSurface`) with `@a2ui/react v0.15.0` & `@a2ui/web_core v0.15.0` streamed over `go.temporal.io/sdk/contrib/workflowstreams v0.1.1` topics (`a2ui`, `requester-ui`, `approver-ui`).
* **⏳ Durable Execution (`Temporal SDK v1.51.0`)**: Server-side state, programmatic guardrails, human-in-the-loop (HITL) approval gates, concurrent `workflow.Go` fan-out, live `workflow.GetVersion` schema migration, child workflows, and `SIGKILL` crash recovery.

> 📜 **Prior Work & Related Presentations:**
> * **Prior Work (Python Prototype):** [brucearctor/a2ui-workflow-streams](https://github.com/brucearctor/a2ui-workflow-streams)
> * **Presentation Deck 1 (Talk Slides):** [Google Slides: The Agent Interoperability Stack](https://docs.google.com/presentation/d/12E7tx-6DkCLqqWFvItu_chxdHILgFS69X9Iu0g6FOb8/edit?slide=id.p#slide=id.p) (also served locally as an interactive offline HTML deck at `http://localhost:8000/slides`)
> * **Presentation Deck 2 (Companion Visuals):** [Google Slides: Architecture & Visuals](https://docs.google.com/presentation/d/1bpdmA4iASw5f3rqMmR9joUri--oGDc7iSn9tS7TW3QM/edit?slide=id.p#slide=id.p)

---

## Student-Friendly Modular Architecture (Self-Contained Scenario Packages)

To make the codebase easy for students and engineers to study in isolation, each of the 5 scenarios lives in its own self-contained package under `backend/pkg/` with the exact same 5-file anatomy:

```text
backend/pkg/<scenario>/
├── types.go          # Input/output structs, TaskQueue constant, signal/topic names
├── messages.go       # Pure A2UI v1.0 surface & data-model builders (zero Temporal imports)
├── activities.go     # (or agent.go) ADK hierarchy + A2A JSON-RPC 2.0 Activity wrappers
├── workflow.go       # Temporal Workflow definition + workflowstreams publishing
├── handler.go        # RegisterRoutes(mux, temporalClient, streamSSE) HTTP/SSE endpoints
└── workflow_test.go  # Standalone Temporal TestWorkflowEnvironment unit tests
```

| Package | Scenario | Core Concepts Demonstrated |
| :--- | :--- | :--- |
| `backend/pkg/order` | **Tab 1 — Order Tracking (`/order`)** | Single A2UI v1.0 surface (`order-tracker`), `@path` JSON Pointer deltas, Fragile (in-memory) vs. Durable (`workflowstreams`) `SIGKILL` crash comparison. |
| `backend/pkg/approval` | **Tab 2 — Dual-Surface & Quorum (`/approval`)** | 1 workflow driving 2 role-specific A2UI topics (`requester-ui` & `approver-ui`), plus **Room Quorum Mode** (`RequiredApprovals: 3`) with live vote tallies. |
| `backend/pkg/concierge` | **Tab 3 — "Durable Eats" AI Concierge (`/ai-concierge`)** | 4-agent Go ADK hierarchy, remote A2A `:9001` & `:9002` calls via `googleadk.ActivityAsTool`, Programmatic Dietary Guardrail, >\$100 Manager HITL Gate, Child Workflow (`OrderWorkflow`). |
| `backend/pkg/incident` | **Tab 4 — "CloudGuard" AI SRE (`/incident-sre`)** | 4-agent Go ADK SRE hierarchy, remote A2A `:9003` (Telemetry) & `:9004` (SecOps Blast-Radius), Change-Freeze HITL Gate, Canary Rollback `RB-402`. |
| `backend/pkg/council` | **Tab 5 — Parallel Council & v2 Upgrade (`/council`)** | Concurrent `workflow.Go` A2A specialist fan-out (`:9003` + `:9004` + `:9001`) + Live `v1 → v2` A2UI surface schema migration via `workflow.GetVersion` & `upgrade_schema` signal. |

---

## Interactive Stage Features

* **⚡ Simulate A2A 503 Outage Toggle (`POST /api/a2a/chaos`)**: Inject HTTP `503 Service Unavailable` across all 4 remote A2A specialist servers (`:9001`–`:9004`) to watch Temporal's Activity Retry policy back off and automatically recover once healed.
* **🔬 Protocol X-Ray Inspector**: Expandable wire-level inspector in Tabs 3, 4, and 5 (and inside the `D` DevTools drawer) showing the remote A2A `/.well-known/agent-card.json`, JSON-RPC 2.0 `message/send` request/response, Temporal `Attempt #N`, and emitted A2UI v1.0 envelopes.
* **🗳️ Audience Quorum Mode (Tab 2)**: Toggle between **Single Manager (1 Vote)** and **Room Quorum Mode (3 Votes)** so audience members can scan the QR code and vote together in real time.
* **🔼 Live `v1 ↔ v2` Schema Hot-Swap (Tab 5)**: Press `U` (or click **Hot-Upgrade to v2 Executive Scorecard**) to replace a live `v1` Compact Summary A2UI surface with a `v2` Executive Scorecard without restarting the workflow.

---

## Quick Start

### Option A: Nix Dev Shell (Recommended)

The included `flake.nix` provides Go 1.26, Node 22, Temporal CLI, `cloudflared`, linters, and security scanners:

```bash
# 1. Build Go binaries and install React frontend dependencies
nix develop -c make setup

# 2. Run the automated E2E test suite across all 5 scenarios + A2A 503 chaos
nix develop -c make e2e

# 3. Start the full interactive demo stack
nix develop -c make demo
```

### Option B: Standard Local Environment (Without Nix)

Prerequisites: **Go 1.22+**, **Node.js 20+**, and the **Temporal CLI** (`brew install temporal`).

```bash
# 1. Setup backend and frontends
make setup

# 2. Run tests and start the stack
make e2e
make demo
```

Once started:
- **Presenter UI (5 Scenarios):** `http://localhost:5173`
- **Audience Phone Kiosk:** `http://localhost:5174`
- **Interactive Slide Deck:** `http://localhost:8000/slides`
- **Temporal Web UI:** `http://localhost:8233`
- **A2A Agent Cards:** `http://localhost:9001..9004/.well-known/agent-card.json`

> **100% Conference Wi-Fi Proof:** If `GOOGLE_API_KEY` is set in `.env`, the Go worker uses live `gemini-2.5-flash` with automatic fallback. If unset (or `OFFLINE_MODE=true`), the worker's `ResilientModelFactory` drives all ADK sub-agents, A2A JSON-RPC calls, A2UI surfaces, guardrails, and HITL gates deterministically offline.

---

## Hands-On Audience Experiments (Try It Yourself!)

### Experiment 1: The Fragile vs. Durable Worker Crash (`K`)
1. Open `http://localhost:5173` and select **Tab 1: Order Tracking**.
2. Click **💔 Fragile Mode** and submit an order. While the tracker is moving, click **💀 Kill Worker** (or press `K`). Notice the in-memory stream shatters; refreshing or reconnecting results in permanent session loss.
3. Switch to **🔒 Durable Mode**, submit an order, and click **💀 Kill Worker (`K`)**.
4. Click **Restart Worker** — the workflow automatically recovers, streams missing messages via `Last-Event-ID`, and delivers the order with zero data loss!

### Experiment 2: Mobile Quorum Voting (Audience Phone Kiosk)
1. Select **Tab 2: Dual-Surface Approval**.
2. Switch mode to **Room Quorum Mode (3 Votes)** and select **🎓 Campus AI Hackathon Grant**.
3. In a second terminal, run `./scripts/tunnel.sh 5174` (or open `http://localhost:5174` in an incognito window or phone).
4. Click **Share Live** to reveal the QR code, scan with your phone, and submit votes. Watch live vote tallies advance on both the presenter screen and mobile phone in real time!

### Experiment 3: The Programmatic Dietary Guardrail & Replay
1. Select **Tab 3: Durable Eats AI Concierge**.
2. Click prompt 1 (*"Show me the menu"*) → sub-agent handoff to `📋 Menu Advisor`.
3. Click prompt 2 (*"Order 4 Tacos al Pastor"*) → sub-agent handoff to `🛒 Cart Manager`.
4. Click **Confirm Order** on the A2UI card → observe how the **Programmatic Guardrail** blocks checkout in code until the remote **A2A Dietary Advisor (`:9002`)** verifies allergen safety and spawns the live `OrderWorkflow` child workflow!

### Experiment 4: CloudGuard AI SRE War Room & Rollback Gate
1. Select **Tab 4: CloudGuard AI SRE**.
2. Follow the 3 investigation steps:
   - Step 1: Telemetry audit via remote **A2A `:9003`** (identifies canary regression).
   - Step 2: Blast-radius evaluation via remote **A2A `:9004`** (recommends Runbook `RB-402`).
   - Step 3: Change-Freeze HITL Gate — click **✅ Authorize Rollback** to execute canary remediation.

### Experiment 5: A2A 503 Chaos Retry & Live Schema Hot-Upgrade (`U`)
1. Select **Tab 5: Parallel Council**.
2. Click **⚡ Simulate A2A 503 Outage** to put all 4 remote A2A microservices into HTTP 503 failure mode.
3. Click **🚀 Fan-Out Parallel A2A Council** — watch 3 concurrent activities safely retry with exponential backoff.
4. Click **Heal A2A Cluster** — all 3 audits immediately succeed!
5. Press `U` (or click **Hot-Upgrade to v2 Executive Scorecard**) to swap the live A2UI surface from `v1` to `v2` mid-flight without workflow restart.

---

## How Hosting & The "Moment of Cutoff" Work

### The Architecture: Live Audience Participation

```text
┌────────────────────────┐      ┌────────────────────────┐
│   Presenter Laptop /   │      │   Audience Mobile /    │
│      Cloud Server      │      │     Second Screen      │
│                        │      │                        │
│  [Go Worker :8000]     │      │   [Phone Browser]      │
│  [Temporal :7233]      │      │          ▲             │
│  [A2A :9001..:9004]    │      │          │ HTTPS / SSE │
│  [Vite Kiosk :5174]◄───┼──────┼──[Cloudflare Tunnel]   │
└────────────────────────┘      └────────────────────────┘
```

1. **Vite Audience Proxy (`:5174`):** The audience kiosk dev server listens on `0.0.0.0:5174` and proxies all `/api/*` SSE and POST requests to the Go backend (`:8000`).
2. **Tunneling for Mobile Phones:** Running `./scripts/tunnel.sh 5174` creates a secure public HTTPS endpoint (e.g. `https://xxx.trycloudflare.com`). Setting `PUBLIC_URL` in `.env` (or via the **Share Live** popover) ensures all QR codes encode this reachable URL.
3. **What Happens at the Moment of Cutoff:**
   - When the worker is crashed (`SIGKILL`), the Go BFF and Temporal server continue running.
   - Audience mobile phones maintain or automatically reconnect their EventSource connections.
   - Because state is journaled durably in Temporal (not in process memory), restarting the worker replays in-flight workflows to their exact pre-crash state.
   - Late-joining or disconnected phones connect at offset `0` and reconstruct the full A2UI component tree and data model in milliseconds.
4. **Cloud / Remote Hosting:** If deploying to a cloud VM (GCP Compute Engine, AWS EC2, Fly.io, or GitHub Codespaces), set `PORT=8000` and `PUBLIC_URL=https://<your-domain-or-ip>:5174`. Both Vite configurations support `.trycloudflare.com` and custom hosts out of the box.
