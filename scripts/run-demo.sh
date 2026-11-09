#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$ROOT_DIR"

if [ ! -f .env ] && [ -f .env.example ]; then
    cp .env.example .env
fi

if [ -f .env ]; then
    set -a; source .env; set +a
fi

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

cleanup() {
    echo -e "\n${YELLOW}Shutting down services...${NC}"
    kill ${TEMPORAL_PID:-} ${A2A_PID:-} ${BFF_PID:-} ${FRONTEND_PID:-} ${AUDIENCE_PID:-} 2>/dev/null || true
    wait 2>/dev/null || true
    echo -e "${GREEN}Done.${NC}"
}
trap cleanup EXIT INT TERM

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  The Agent Interoperability Stack (All-Go Edition)${NC}"
echo -e "${CYAN}  ADK-Go v2.7.1 × A2A v2.8.0 × A2UI v1.0 × Temporal v1.50.0${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

PORTS="7233 8000 8233 9001 9002 9003 9004 5173 5174"
STALE_PIDS=$(lsof -ti:${PORTS// /,} 2>/dev/null || true)
if [ -n "$STALE_PIDS" ]; then
    echo -e "\n${YELLOW}⚠ Cleaning up stale processes on ports: ${PORTS}${NC}"
    echo "$STALE_PIDS" | xargs kill -9 2>/dev/null || true
    sleep 1
fi

echo -e "\n${GREEN}[0/5]${NC} Building Go binaries (server, worker, a2aservers)..."
cd "$ROOT_DIR/backend"
mkdir -p bin
go build -o bin/worker ./cmd/worker
go build -o bin/a2aservers ./cmd/a2aservers
go build -o bin/server ./cmd/server

echo -e "${GREEN}[1/5]${NC} Starting Temporal dev server (:7233, UI :8233)..."
temporal server start-dev --log-level error &
TEMPORAL_PID=$!
sleep 3

echo -e "${GREEN}[2/5]${NC} Starting 4 Go A2A JSON-RPC 2.0 Agents (:9001, :9002, :9003, :9004)..."
"$ROOT_DIR/backend/bin/a2aservers" &
A2A_PID=$!
sleep 1

echo -e "${GREEN}[3/5]${NC} Starting Go HTTP/SSE BFF on :8000 (auto-spawns ./bin/worker)..."
cd "$ROOT_DIR/backend"
./bin/server &
BFF_PID=$!
sleep 2

echo -e "${GREEN}[4/5]${NC} Starting React presenter UI on :5173..."
cd "$ROOT_DIR/frontend-react"
npm run dev &
FRONTEND_PID=$!

echo -e "${GREEN}[5/5]${NC} Starting Audience phone kiosk UI on :5174..."
cd "$ROOT_DIR/frontend-audience"
npm run dev &
AUDIENCE_PID=$!

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  All services running!${NC}"
echo -e ""
echo -e "  Presenter UI:  ${CYAN}http://localhost:5173${NC}  (4 scenarios)"
echo -e "  Audience UI:   ${CYAN}http://localhost:5174${NC}  (QR phone kiosk)"
echo -e "  Slide Deck:    ${CYAN}http://localhost:8000/slides${NC}  (Talk presentation)"
echo -e "  Go BFF API:    ${CYAN}http://localhost:8000${NC}  (SSE + WorkerManager)"
echo -e "  A2A Agents:    ${CYAN}http://localhost:9001..9004/.well-known/agent-card.json${NC}"
echo -e "  Temporal UI:   ${CYAN}http://localhost:8233${NC}  (Event History & Replay)"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

wait
