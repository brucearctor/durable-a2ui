#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "🔧 [1/3] Building Go backend binaries (server, worker, a2aservers)..."
cd "$ROOT_DIR/backend"
mkdir -p bin
go build -o bin/server ./cmd/server
go build -o bin/worker ./cmd/worker
go build -o bin/a2aservers ./cmd/a2aservers

echo "📦 [2/3] Installing React presenter dependencies..."
cd "$ROOT_DIR/frontend-react"
npm install

echo "📱 [3/3] Installing Audience kiosk dependencies..."
cd "$ROOT_DIR/frontend-audience"
npm install

echo "✅ Setup complete! Run ./scripts/run-demo.sh to launch the stack."
