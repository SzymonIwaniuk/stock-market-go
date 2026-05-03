#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

PORT="${1:-8080}"
MAX_RETRIES=30
RETRY_INTERVAL=2

echo "=== Stock Market Go — Deploy ==="
echo "Port: $PORT"
echo ""

echo "[1/3] Building and starting containers..."
PORT="$PORT" docker compose up --build -d

echo "[2/3] Waiting for service to be healthy..."
for i in $(seq 1 "$MAX_RETRIES"); do
    if curl -sf "http://localhost:${PORT}/health" > /dev/null 2>&1; then
        echo "  Health check passed (attempt $i/$MAX_RETRIES)"
        echo ""
        echo "[3/3] Service is ready!"
        echo "  URL: http://localhost:${PORT}"
        echo ""
        echo "  Useful commands:"
        echo "    docker compose logs -f     — follow logs"
        echo "    docker compose down -v     — stop and clean up"
        echo "    make e2e-test PORT=$PORT   — run end-to-end tests"
        exit 0
    fi
    echo "  Waiting... ($i/$MAX_RETRIES)"
    sleep "$RETRY_INTERVAL"
done

echo ""
echo "ERROR: Service failed to become healthy after $((MAX_RETRIES * RETRY_INTERVAL))s"
echo "Container logs:"
docker compose logs
exit 1
