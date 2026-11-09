# The Agent Interoperability Stack: ADK + A2A + A2UI — and Why Durability Changes Everything (All-Go Edition)

This repository powers the 35–40 minute live conference talk and demo for both **University CS Students** and **GDG DevFest 2026 Ciudad de México (CDMX)**.

Every backend component—workflows, multi-agent orchestration, remote A2A servers, A2UI v1.0 message builders, and the HTTP/SSE BFF—is written in **100% Go**:

* **🧠 Agent Orchestration (`ADK-Go v2.7.1`)**: `google.golang.org/adk/v2` + `go.temporal.io/sdk/contrib/googleadk v0.3.0` (`googleadk.NewPlugin`, `googleadk.NewContext`, `googleadk.NewModel`, `googleadk.ActivityAsTool`, `googleadk.ExportSession` / `ImportSession`).
* **🔗 Agent-to-Agent Protocol (`A2A-Go v2.8.0`)**: `github.com/a2aproject/a2a-go/v2` (`a2asrv.NewJSONRPCHandler`, `a2asrv.NewStaticAgentCardHandler`, `agentcard.DefaultResolver`, `a2aclient.NewFromCard`) running 4 real HTTP JSON-RPC 2.0 servers on `:9001`–`:9004`.
* **🎨 Agent-to-UI Protocol (`A2UI v1.0`)**: Declarative component trees (`createSurface`, `updateComponents`, `updateDataModel`) with `@a2ui/react v0.15.0` & `@a2ui/web_core v0.15.0` streamed over `go.temporal.io/sdk/contrib/workflowstreams v0.1.1` topics (`a2ui`, `requester-ui`, `approver-ui`).
* **⏳ Durable Execution (`Temporal SDK v1.50.0`)**: Server-side state, programmatic guardrails, human-in-the-loop (HITL) approval gates, child workflows, and live `SIGKILL` crash recovery.

---

## 4 Live Demo Scenarios (Presenter UI on `:5173`)

1. **Tab 1 — 🍕 Order Tracking (`/order`)**
   * Single A2UI v1.0 surface (`order-tracker`) driven by `order.OrderWorkflow`.
   * Side-by-side **Fragile (In-Memory)** vs. **Durable (Temporal)** toggle: click **💀 Kill Worker** mid-order to watch Fragile lose all state while Durable resumes from its exact step upon **🔄 Restart Worker**.
   * Audience QR code sync via `:5174`.
2. **Tab 2 — 📋 Dual-Surface Approval (`/approval`)**
   * One `approval.ApprovalWorkflow` publishing to two `workflowstreams` topics (`requester-ui` and `approver-ui`).
   * Includes 1-click presets for **🇲🇽 DevFest CDMX Cloud & Speaker Grant (\$2,135)** and **🎓 University Campus GPU & Hackathon Grant (\$2,135)**.
3. **Tab 3 — 🤖 "Durable Eats" AI Concierge (`/ai-concierge`)**
   * 4-agent ADK hierarchy (`concierge` → `menu_advisor`, `cart_manager`, `order_confirmer`).
   * Calls 2 live Go A2A JSON-RPC 2.0 servers (`:9001` **Restaurant & Campus Spot Finder** featuring *Taquería Orinoco CDMX* & *Boba & Byte Campus Lab*, and `:9002` **Dietary & Allergy Safety Advisor**) wrapped via `googleadk.ActivityAsTool`.
   * Features a **Programmatic Dietary Safety Guardrail** (blocks `confirm_order` in code until `:9002` A2A dietary verification completes), a **\$100 Manager HITL Approval Gate**, and a **Temporal Child Workflow** handoff to `OrderWorkflow`.
4. **Tab 4 — 🚨 "CloudGuard" AI SRE Incident Commander (`/incident-sre`)**
   * 4-agent ADK SRE hierarchy (`incident_commander` → `telemetry_investigator`, `mitigation_planner`, `remediation_executor`).
   * Calls 2 live Go A2A JSON-RPC 2.0 servers (`:9003` **CloudGuard Telemetry & Canary Diagnostics** and `:9004` **SecOps Blast-Radius & Change-Freeze Policy**).
   * Enforces a **Programmatic SecOps Guardrail** and an **Incident Commander HITL Approval Gate** before executing GKE canary rollback `RB-402`.

---

## Quick Start (Nix Dev Shell)

```bash
nix develop -c ./scripts/setup.sh
nix develop -c ./scripts/run-demo.sh
```

> **100% Conference Wi-Fi Proof:** If `GOOGLE_API_KEY` is set in `.env`, the Go worker uses live `gemini-2.5-flash` with automatic fallback. If unset (or `OFFLINE_MODE=true`), the worker's `ResilientModelFactory` drives all 4 ADK sub-agents, A2A JSON-RPC calls, A2UI surfaces, guardrails, and HITL gates deterministically offline.

---

## 35–40 Minute Speaker Run-of-Show

* **00:00 – 05:00 | The "Fragile Agent" Problem**: Why demos work on localhost and die in production; introducing the 4-layer stack (**ADK** = Brain, **A2A** = Remote Agent Mesh, **A2UI** = Generative UI Surface, **Temporal** = Durable Backbone).
* **05:00 – 12:00 | Act 1 — Tab 1 (`🍕 Order Tracking`) & Tab 2 (`📋 Dual-Surface Approval`)**:
  * Show A2UI v1.0 JSON envelopes (`createSurface`, `updateComponents`, `updateDataModel`) in the **DevTools (`D`)** panel.
  * Switch between **Fragile** and **Durable** mode and click **💀 Kill Worker**.
  * Open Tab 2, toggle between the **🎓 Campus Hackathon GPU Grant** and **🇲🇽 DevFest CDMX Cloud Grant** presets, and show one workflow driving two role-specific UIs (`requester-ui` and `approver-ui`).
* **12:00 – 25:00 | Act 2 — Tab 3 (`🤖 Durable Eats Concierge`)**:
  * Click **Show me the menu** → watch `concierge` transfer to `menu_advisor` and render an interactive A2UI menu card with *Taquería Orinoco CDMX* Tacos al Pastor and *Boba & Byte* Brown Sugar Boba.
  * Click **DevFest Team Feast: 10 Tacos al Pastor** → watch `cart_manager` update the durable server-side cart (`\$115.00`).
  * Click **✅ Confirm Order** on the A2UI cart card → watch the **Programmatic Safety Guardrail** block `confirm_order`, automatically call the `:9002` **Dietary Safety A2A Agent** over JSON-RPC 2.0, and then trigger the **\$100 Manager HITL Approval Banner**.
  * Click **💀 Kill Worker** while the Manager Approval Banner is waiting! Restart the worker, click **Approve**, and watch `ConciergeWorkflow` spawn `OrderWorkflow` as a **Child Workflow**.
* **25:00 – 33:00 | Act 3 — Tab 4 (`🚨 CloudGuard AI SRE`)**:
  * Show the same Go `googleadk` + `a2a-go` + `workflowstreams` architecture solving a Tier-0 GKE + AlloyDB production incident (`checkout-api-canary-v2.14.0-rc3`).
  * Press `F` (**Fast Forward Step**) through Telemetry (`:9003`), SecOps Policy (`:9004`), and Incident Commander HITL sign-off to execute Canary Rollback `RB-402`.
* **33:00 – 38:00 | Under the Hood in Go & Q&A**:
  * Highlight `googleadk.NewContext(ctx)` (deterministic time, UUIDs, and tool fan-out), `googleadk.NewPlugin(...)`, `googleadk.ActivityAsTool(...)`, and `workflowstreams.NewTopic[map[string]any]`.
