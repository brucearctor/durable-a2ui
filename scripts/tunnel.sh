#!/usr/bin/env bash
set -euo pipefail
PORT="${1:-5174}"
echo "🌐 Starting Cloudflare Tunnel → http://localhost:${PORT}"
cloudflared tunnel --url "http://localhost:${PORT}"
