# Speaker Run-of-Show & Stage Guide

**Talk Title:** *The Agent Interoperability Stack: ADK + A2A + A2UI — and Why Durability Changes Everything*

---

## 1. Pre-Stage Checklist

1. **Choose Online vs. Offline Mode**
   - **Live Gemini (`gemini-2.5-flash`):** Export `GOOGLE_API_KEY` in `.env` (automatically falls back to the deterministic multi-agent engine if venue Wi-Fi drops).
   - **100% Offline Stage-Safe Mode:** Set `OFFLINE_MODE=true` in `.env` to run all 4 ADK sub-agent transfers, A2A JSON-RPC 2.0 calls (`:9001`–`:9004`), programmatic guardrails, and A2UI `v1.0` surfaces with zero external network dependency.
2. **Verify & Start the Full Stack**
   ```bash
   nix develop -c make e2e
   nix develop -c make demo
   ```
3. **Audience Mobile QR & Tunnel Setup (For Live Voting & Mobile Cutoff Demo)**
   - In a second terminal, start the Cloudflare Tunnel for the audience kiosk (port 5174):
     ```bash
     nix develop -c ./scripts/tunnel.sh 5174
     ```
   - Copy the public HTTPS URL from terminal output (e.g., `https://random-subdomain.trycloudflare.com`).
   - Set `PUBLIC_URL=https://random-subdomain.trycloudflare.com` in `.env` (or click **Share Live** in the UI and click **Set Public Tunnel URL** to paste it).
   - Now any QR code projected on stage is 100% scannable from mobile phones in the auditorium!
4. **Open Browser Tabs (or Run Everything Inside the Slide Deck)**
   - **Tab A — Slide Deck:** `http://localhost:8000/slides`
     - Press `L` on any demo slide to launch the live demo or Temporal Web UI inline.
     - Press `S` to toggle the Speaker Notes drawer.
     - Press `G` to open the Slide Overview grid.
   - **Tab B — Presenter UI:** `http://localhost:5173`
   - **Tab C — Audience Phone Kiosk:** `http://localhost:5174` (or public tunnel URL)
   - **Tab D — Temporal Web UI:** `http://localhost:8233`

---

## 2. Presenter Keyboard Shortcuts

### Slide Deck (`http://localhost:8000/slides`)

| Key | Action |
| :--- | :--- |
| `→` / `Space` | Next slide |
| `←` | Previous slide |
| `L` | Open / close Embedded Live Demo & Temporal UI modal |
| `G` | Open / close Slide Overview grid |
| `S` | Toggle Speaker Notes drawer |

### Live Demo UI (`http://localhost:5173`)

| Key | Action |
| :--- | :--- |
| `1` / `2` / `3` / `4` / `5` | Switch between Scenarios 1–5 |
| `D` | Toggle **DevTools Panel** (live A2UI `v1.0` frames, Protocol X-Ray, token/cost metrics, ADK & A2A trace) |
| `K` | **Kill Worker (`SIGKILL`)** — crashes `./bin/worker` process to prove durability |
| `F` | **Fast-Forward** next workflow stage or next demo prompt |
| `U` | **Hot-Upgrade Schema (`v1 ↔ v2`)** in Parallel A2A Council (Tab 5) |
| `R` | Trigger Rush Order signal (Tab 1) |
| `C` | Cancel Order signal (Tab 1) |
| `A` / `X` | Approve (`A`) or Reject (`X`) in Dual-Surface Approval (Tab 2) & Parallel Council (Tab 5) |
| `N` | Reset / Start New Session |

---

## 3. Presentation & Demo Choreography

### Act I: The 90% Agent Demo Trap (Slides 1–3)
- **Hook:** *"Who here has built an AI agent that worked on `localhost`? Keep your hand up if it survived a `kill -9` pod restart or a user refreshing their browser mid-tool-call."*
- **Core Thesis:**
  - **ADK (`google.golang.org/adk/v2 v2.7.1`)** gives your agent a **brain**.
  - **A2A (`github.com/a2aproject/a2a-go/v2 v2.8.0`)** gives it **colleagues**.
  - **A2UI (`v1.0` + `@a2ui/react v0.15.0`)** gives it a **face**.
  - **Temporal (`v1.51.0` + `googleadk v0.3.0` + `workflowstreams v0.1.1`)** gives it a **spine**.

### Act II-A: Declarative A2UI v1.0 + Durable Streams (Slides 4–5 + Demo 1)
- **Concept:** Send the `A2UI v1.0` component tree once (`createSurface` + `updateComponents`), then stream tiny JSON Pointer (`@path`) deltas via `updateDataModel`.
- **Live Demo 1 (Tab 1 — Order Tracking):**
  1. Start an order in **🔒 Durable Mode** and press `D` to show `createSurface` (`version: "1.0"`) and `@path` bindings in DevTools.
  2. Switch to **💔 Fragile Mode**, start an order, and click **💀 Kill Worker (`K`)** — watch the in-memory session shatter permanently.
  3. Switch back to **🔒 Durable Mode**, kill the worker (`K`), then click **Restart Worker** — the UI resumes from the exact offset via `Last-Event-ID` with zero data loss.

### Act II-B: Multi-Persona Surfaces & Human-in-the-Loop (Slides 6–7 + Demo 2)
- **Concept:** One Temporal workflow publishing to two isolated `workflowstreams` topics (`requester-ui` and `approver-ui`). The requester never even receives the `Approve`/`Reject` button components over the wire.
- **Live Demo 2 (Tab 2 — Dual-Surface Approval):**
  1. Select **☁️ Production AI & GPU Cluster Grant** (or **🎓 Campus AI Hackathon Grant**) and submit.
  2. Point out the side-by-side **Requester** (read-only tracker) and **Approver** (interactive buttons) A2UI surfaces.
  3. While waiting for approval, press `K` (**Kill Worker**), then restart it — proving a human approval gate can wait across server restarts without losing state.
  4. Press `A` (or click **✅ Approve**) and watch both surfaces transition to `Approved` simultaneously.

### Act III: Multi-Agent ADK + Remote A2A + Guardrails (Slides 8–11 + Demo 3)
- **Concept:**
  - `googleadk.NewContext(ctx)` bridges Temporal's deterministic clock, RNG, and coroutines into ADK.
  - Fast state mutations (`get_menu`, `add_to_cart`, `confirm_order`) run in-workflow via `functiontool.New`; network I/O (`a2a_find_restaurants` on `:9001`, `a2a_check_dietary_safety` on `:9002`) runs as durable Temporal Activities via `googleadk.ActivityAsTool`.
  - **Programmatic Guardrail:** Never trust a system prompt alone for safety—`confirm_order` blocks in Go code until `dietaryChecked == true`.
- **Live Demo 3 (Tab 3 — "Durable Eats" AI Concierge):**
  1. Open DevTools (`D`) to show live ADK sub-agent handoffs and token metrics.
  2. Click **"Show me the menu"** → watch `concierge` hand off to `📋 Menu Advisor` and render the A2UI menu surface.
  3. Click **"Order 4 Tacos al Pastor and 2 Horchata Artesanal"** → watch `🛒 Cart Manager` render the interactive A2UI `v1.0` Cart surface.
  4. Click **"✅ Confirm Order"** on the A2UI card → watch the **Programmatic Dietary Guardrail** block checkout, automatically invoke the remote **A2A Dietary Advisor (`:9002`)**, render the green A2UI verification card, and spawn the live `OrderWorkflow` child workflow!
  5. *(Optional Bonus)* Trigger the **>\$100 Team Feast** prompt to show the **Manager HITL Approval Banner** inside chat.

### Act IV: Same Stack, Second Domain — AI SRE Incident Commander (Slides 12–13 + Demo 4)
- **Concept:** Show architectural generality—swap the food catalog for SRE runbooks and `:9003`/`:9004` A2A agents.
- **Live Demo 4 (Tab 4 — "CloudGuard" AI SRE Incident Commander):**
  1. Press `4` to open Tab 4 and press `F` (or click Step 1) to investigate the `checkout-api` `SEV-1` outage.
  2. Watch `📡 Telemetry Investigator` call **A2A `:9003`** and render the A2UI Telemetry & Root-Cause card (`96%` confidence on `checkout-api-canary-v2.14.0-rc3`).
  3. Click **Step 2 (`Evaluate Blast Radius`)** → `🛡️ Mitigation Planner` calls **A2A `:9004`** (`PASS_WITH_HITL_GATE`) and renders Runbook `RB-402`.
  4. Click **Step 3 (`Execute Canary Rollback RB-402`)** → triggers the **Incident Commander HITL Gate**, click **✅ Authorize Rollback**, and watch the post-remediation A2UI verification card confirm `5xx: 18.4% → 0.01%` and `p99: 4,820ms → 42ms`.

### Act V: Parallel A2A Fan-Out (`workflow.Go`) & Live `v1 → v2` Schema Upgrade (Slide 14 + Demo 5)
- **Concept:**
  - **Option A (`workflow.Go` Fan-Out):** Spawn 3 concurrent A2A specialist evaluations (`:9003` Reliability, `:9004` SecOps, `:9001` FinOps) and stream each specialist card onto `council-board` as soon as it finishes.
  - **Option C (`workflow.GetVersion` + Live Signal):** Upgrade an in-flight workflow's A2UI surface from `v1` (Compact Summary) to `v2` (Executive Scorecard) without restarting the workflow or losing state.
- **Live Demo 5 (Tab 5 — Parallel A2A Council & Live Schema Upgrade):**
  1. Press `5` to open Tab 5. Click **⚡ Simulate A2A 503 Outage** so all 4 A2A servers return `503 Service Unavailable`.
  2. Click **🚀 Fan-Out Parallel A2A Council** — point out that all 3 concurrent activities are retrying safely in Temporal while A2A is down.
  3. Click **🔥 A2A 503 Outage Active (Click to Heal)** — all 3 specialists immediately complete! Expand **🔬 Protocol X-Ray Inspector** to show `⚡ Recovered on Attempt #N` alongside the raw `/.well-known/agent-card.json` and JSON-RPC `message/send` payloads.
  4. Press `U` (or click **🔼 Hot-Upgrade to v2 Executive Scorecard**) — watch `council-verdict` hot-swap from `v1` Compact Layout to `v2` Executive Scorecard mid-flight, then press `A` to approve rollout.

### Wrap-Up & Q&A (Slide 15)
- Recap the 4 rules:
  1. **Separate Structure from State (`A2UI v1.0`)**
  2. **Standardize Agent Boundaries (`A2A JSON-RPC 2.0`)**
  3. **Enforce Guardrails in Code, Not Prompts**
  4. **Make Every Turn Durable (`Temporal` + `googleadk` + `workflowstreams`)**
