# Durable A2UI — Audience & Mobile Phone Kiosk

A dedicated, lightweight, mobile-optimized A2UI client designed for audience members, phones, tablets, and second screens during live demonstrations and presentations.

---

## What It Does

When presenting **The Agent Interoperability Stack (ADK + A2A + A2UI + Temporal)**, the presenter can display a live QR code on screen. Audience members can scan the QR code with their phone cameras to:

1. **Watch Live Generative UI Streams:** Stream declarative A2UI v1.0 message frames (`createSurface`, `updateComponents`, `updateDataModel`) in real time over SSE directly to their mobile browser.
2. **Participate in Quorum Voting (Scenario 2):** In Room Quorum Mode, multiple audience members scan the QR code and vote (`Approve` / `Reject`) simultaneously on their phones, watching live vote tallies advance on stage in real time.
3. **Experience Durability at the Moment of Cutoff:** When the presenter crashes the worker process (`SIGKILL` / `kill -9`), audience phones stay connected or reconnect seamlessly via `Last-Event-ID` without losing UI state, proving Temporal server-side durability on their own devices.

---

## Supported URL Parameters

The kiosk automatically routes to the appropriate scenario stream and API endpoints based on the workflow ID prefix and role:

| Parameter | Example | Scenario / Role |
| :--- | :--- | :--- |
| `?wf=order-<id>` | `?wf=order-a1b2c3d4` | **Tab 1:** Live Order Tracker (`/api/orders/{id}/stream`) |
| `?wf=approval-<id>&role=approver` | `?wf=approval-7e8f90ab&role=approver` | **Tab 2:** Approver Surface with interactive voting buttons |
| `?wf=approval-<id>&role=requester` | `?wf=approval-7e8f90ab&role=requester` | **Tab 2:** Requester Surface (read-only live tracker) |
| `?wf=concierge-<id>` | `?wf=concierge-3c4d5e6f` | **Tab 3:** Durable Eats AI Concierge conversation & cards |
| `?wf=incident-<id>` | `?wf=incident-9a8b7c6d` | **Tab 4:** CloudGuard AI SRE War Room & rollback approval |
| `?wf=council-<id>` | `?wf=council-5e6f7a8b` | **Tab 5:** Parallel A2A Council & live v1 ↔ v2 schema swap |

---

## Local Development & Audience Hosting

### 1. Running Locally (Port 5174)

```bash
# Started automatically by `make demo` (./scripts/run-demo.sh), or run standalone:
cd frontend-audience
npm install
npm run dev
```

The Vite dev server runs on `http://localhost:5174` (bound to `0.0.0.0`) and proxies all `/api/*` requests to the Go BFF server at `http://localhost:8000`.

### 2. Making It Scannable from Mobile Phones

Mobile phones in the audience cannot resolve `http://localhost:5174`. To allow audience members to scan the QR code:

```bash
# In a separate terminal, expose the audience port via Cloudflare Tunnel:
./scripts/tunnel.sh 5174
```

Copy the generated public HTTPS URL (e.g., `https://random-subdomain.trycloudflare.com`) and either:
- Set `PUBLIC_URL=https://random-subdomain.trycloudflare.com` in `.env` before running `make demo`, **or**
- Click **"Share Live"** in the Presenter UI and click **"Set Public Tunnel URL"** to paste it directly into the QR popover.

Now all generated QR codes will encode the public URL, allowing every phone in the room to participate!
