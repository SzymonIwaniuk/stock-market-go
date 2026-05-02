#!/usr/bin/env bash
set -euo pipefail

PORT=${1:?"Usage: ./start.sh <PORT>"}
export PORT

docker compose up --build -d

echo "Stock market service available at http://localhost:${PORT}"
