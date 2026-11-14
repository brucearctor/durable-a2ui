#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "=== [0/7] Building Go binaries ==="
make build

E2E_TMP="$(mktemp -d /tmp/durable-a2ui-e2e.XXXXXX)"
TEMPORAL_PID=""
A2A_PID=""
SERVER_PID=""

cleanup() {
  echo "=== Cleaning up E2E processes ==="
  [[ -n "$SERVER_PID" ]] && kill "$SERVER_PID" 2>/dev/null || true
  [[ -n "$A2A_PID" ]] && kill "$A2A_PID" 2>/dev/null || true
  [[ -n "$TEMPORAL_PID" ]] && kill "$TEMPORAL_PID" 2>/dev/null || true
  pkill -f "bin/worker" 2>/dev/null || true
  rm -rf "$E2E_TMP"
}
trap cleanup EXIT

echo "=== [1/7] Starting Temporal dev server, A2A servers (:9001-:9004), and BFF (:18000, auto-spawning worker) ==="
STALE_A2A=$(lsof -ti:9001,9002,9003,9004,17233,18000 2>/dev/null || true)
if [[ -n "$STALE_A2A" ]]; then
  echo "$STALE_A2A" | xargs kill -9 2>/dev/null || true
  sleep 0.5
fi

temporal server start-dev --port 17233 --ui-port 18233 --http-port 17243 --metrics-port 0 --log-level error >"$E2E_TMP/temporal.log" 2>&1 &
TEMPORAL_PID=$!

for _ in $(seq 1 30); do
  if nc -z 127.0.0.1 17233 2>/dev/null; then
    break
  fi
  sleep 0.5
done
nc -z 127.0.0.1 17233

./backend/bin/a2aservers >"$E2E_TMP/a2a.log" 2>&1 &
A2A_PID=$!

PORT=18000 TEMPORAL_ADDRESS=localhost:17233 ./backend/bin/server >"$E2E_TMP/server.log" 2>&1 &
SERVER_PID=$!

for _ in $(seq 1 20); do
  if curl -sf http://localhost:18000/api/health >/dev/null 2>&1 && curl -sf http://localhost:9001/.well-known/agent-card.json >/dev/null 2>&1; then
    break
  fi
  sleep 0.3
done

echo "=== [2/7] Verifying Health, Slides, and A2A v2.8 Agent Cards ==="
curl -sf http://localhost:18000/api/health | grep -q '"status":"ok"'
curl -sfL http://localhost:18000/slides | grep -q "The Agent Interoperability Stack"
for port in 9001 9002 9003 9004; do
  curl -sf "http://localhost:${port}/.well-known/agent-card.json" | grep -q '"protocolVersion"'
done
echo "  -> Health, Slides, and 4 A2A Agent Cards verified!"
sleep 1.0

echo "=== [3/7] Scenario 1: Order Tracking (Durable + Fragile) ==="
ORDER_WF=$(curl -sf -X POST http://localhost:18000/api/orders \
  -H "Content-Type: application/json" \
  -d '{"customer":"E2E Alice","items":[{"name":"Tacos al Pastor","qty":2,"price":4.50}]}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["workflow_id"])')
sleep 0.5
for _ in 1 2 3 4; do
  curl -sf -X POST "http://localhost:18000/api/orders/${ORDER_WF}/signal/fast_forward" >/dev/null
  sleep 0.4
done
curl -sfN --max-time 2 "http://localhost:18000/api/orders/${ORDER_WF}/stream" >"$E2E_TMP/order_sse.txt" || true
grep -q "Delivered!" "$E2E_TMP/order_sse.txt"

FRAG_ID=$(curl -sf -X POST http://localhost:18000/api/fragile/orders \
  -H "Content-Type: application/json" \
  -d '{"customer":"Fragile Bob","items":[{"name":"Pizza","qty":1,"price":15.0}]}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["order_id"])')
curl -sfN --max-time 2 "http://localhost:18000/api/fragile/orders/${FRAG_ID}/stream" >"$E2E_TMP/fragile_sse.txt" || true
grep -q "createSurface" "$E2E_TMP/fragile_sse.txt"
echo "  -> Scenario 1 verified ($ORDER_WF)!"

echo "=== [4/7] Scenario 2: Dual-Surface Approval with Room Quorum Mode (2 Votes) ==="
APP_WF=$(curl -sf -X POST http://localhost:18000/api/approvals \
  -H "Content-Type: application/json" \
  -d '{"requester":"Alex Rivera","title":"University GPU Grant","justification":"Hackathon","scenario":"campus_hackathon","required_approvals":2,"items":[{"description":"L4 GPU Credits","amount":950,"category":"Cloud Compute"}]}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["workflow_id"])')
sleep 0.5
curl -sf -X POST "http://localhost:18000/api/approvals/${APP_WF}/signal/approve" \
  -H "Content-Type: application/json" -d '{"note":"Vote 1"}' >/dev/null
sleep 0.5
curl -sfN --max-time 2 "http://localhost:18000/api/approvals/${APP_WF}/stream/requester" >"$E2E_TMP/app_vote1.txt" || true
grep -q "1 / 2" "$E2E_TMP/app_vote1.txt"

curl -sf -X POST "http://localhost:18000/api/approvals/${APP_WF}/signal/approve" \
  -H "Content-Type: application/json" -d '{"note":"Vote 2"}' >/dev/null
sleep 0.5
curl -sfN --max-time 2 "http://localhost:18000/api/approvals/${APP_WF}/stream/requester" >"$E2E_TMP/app_vote2.txt" || true
grep -q "Approved by Room Quorum" "$E2E_TMP/app_vote2.txt"
echo "  -> Scenario 2 Dual-Surface + Quorum Voting verified ($APP_WF)!"

echo "=== [5/7] Scenario 3: Durable Eats AI Concierge (A2A :9001/:9002 + Guardrail + HITL + Worker Kill/Replay) ==="
CONC_WF=$(curl -sf -X POST http://localhost:18000/api/concierge \
  -H "Content-Type: application/json" -d '{"customer_name":"Carlos"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["workflow_id"])')
sleep 0.8
curl -sf -X POST "http://localhost:18000/api/concierge/${CONC_WF}/message" \
  -H "Content-Type: application/json" -d '{"text":"Team Feast: 10 Tacos al Pastor"}' >/dev/null
sleep 1.2
curl -sf -X POST "http://localhost:18000/api/concierge/${CONC_WF}/message" \
  -H "Content-Type: application/json" -d '{"text":"Confirm my order"}' >/dev/null
sleep 1.5

# Kill worker mid-workflow and restart via admin API
curl -sf -X POST http://localhost:18000/api/admin/kill-worker >/dev/null
curl -sf -X POST http://localhost:18000/api/admin/start-worker >/dev/null
sleep 1.5

curl -sf -X POST "http://localhost:18000/api/concierge/${CONC_WF}/approve" \
  -H "Content-Type: application/json" -d '{"decision":"approved"}' >/dev/null
sleep 1.5
curl -sfN --max-time 2 "http://localhost:18000/api/concierge/${CONC_WF}/stream" >"$E2E_TMP/conc_sse.txt" || true
grep -qi "Order confirmed" "$E2E_TMP/conc_sse.txt"
grep -q "agent_card" "$E2E_TMP/conc_sse.txt"
echo "  -> Scenario 3 + Protocol X-Ray + Worker Replay verified ($CONC_WF)!"

echo "=== [6/7] Scenario 4: CloudGuard AI SRE Incident Commander (A2A :9003/:9004 + HITL Rollback) ==="
INC_WF=$(curl -sf -X POST http://localhost:18000/api/incidents \
  -H "Content-Type: application/json" \
  -d '{"commander_name":"SRE Lead","service_name":"checkout-api"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["workflow_id"])')
sleep 0.6
for _ in 1 2 3; do
  curl -sf -X POST "http://localhost:18000/api/incidents/${INC_WF}/signal/fast_forward" >/dev/null
  sleep 1.0
done
curl -sf -X POST "http://localhost:18000/api/incidents/${INC_WF}/approve" \
  -H "Content-Type: application/json" -d '{"decision":"approved"}' >/dev/null
sleep 1.2
curl -sfN --max-time 2 "http://localhost:18000/api/incidents/${INC_WF}/stream" >"$E2E_TMP/inc_sse.txt" || true
grep -q "RB-402 Executed Successfully" "$E2E_TMP/inc_sse.txt"
echo "  -> Scenario 4 verified ($INC_WF)!"

echo "=== [7/7] Scenario 5: Parallel A2A Council + A2A 503 Chaos Recovery + Live v1->v2 Schema Migration ==="
curl -sf -X POST http://localhost:18000/api/a2a/chaos \
  -H "Content-Type: application/json" -d '{"outage":true}' >/dev/null

COUNCIL_WF=$(curl -sf -X POST http://localhost:18000/api/council \
  -H "Content-Type: application/json" \
  -d '{"architect":"Priya","release_name":"checkout-api v2.14.0","environment":"us-central1","schema_version":1}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["workflow_id"])')

# Let Temporal retry against HTTP 503 for 1.5s, then heal A2A cluster
sleep 1.5
curl -sf -X POST http://localhost:18000/api/a2a/chaos \
  -H "Content-Type: application/json" -d '{"outage":false}' >/dev/null
sleep 1.8

# Hot-upgrade live A2UI verdict surface from v1 to v2 without restarting workflow
curl -sf -X POST "http://localhost:18000/api/council/${COUNCIL_WF}/upgrade" \
  -H "Content-Type: application/json" -d '{"target_version":2}' >/dev/null
sleep 0.8

curl -sf -X POST "http://localhost:18000/api/council/${COUNCIL_WF}/decide" \
  -H "Content-Type: application/json" -d '{"decision":"approved"}' >/dev/null
sleep 0.8

curl -sfN --max-time 2 "http://localhost:18000/api/council/${COUNCIL_WF}/stream" >"$E2E_TMP/council_sse.txt" || true
grep -q "Schema v2 Executive Scorecard" "$E2E_TMP/council_sse.txt"
grep -q "LAUNCH APPROVED" "$E2E_TMP/council_sse.txt"
echo "  -> Scenario 5 Parallel A2A Council + 503 Retry Recovery + Live v2 Schema Swap verified ($COUNCIL_WF)!"

echo "=== ALL 5 SCENARIOS & CHAOS TESTS PASSED E2E ==="
