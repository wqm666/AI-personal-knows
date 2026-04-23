#!/bin/bash
set -e

DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=========================================="
echo "  Personal-Know Deployment"
echo "=========================================="

# 1. Ensure .env exists
if [ ! -f "$DEPLOY_DIR/.env" ]; then
    echo "[pre] Creating .env from example..."
    cp "$DEPLOY_DIR/.env.example" "$DEPLOY_DIR/.env"
    echo ""
    echo "  Please edit $DEPLOY_DIR/.env"
    echo "  Set POSTGRES_PASSWORD and other secrets."
    echo ""
    echo "  After editing, run this script again."
    exit 0
fi

# 2. Ensure config.json exists
if [ ! -f "$DEPLOY_DIR/config.json" ]; then
    echo "[1/3] Creating config.json from example..."
    cp "$DEPLOY_DIR/config.json.example" "$DEPLOY_DIR/config.json"
    echo ""
    echo "  Please edit $DEPLOY_DIR/config.json"
    echo "  Set your LLM API key and other settings."
    echo ""
    echo "  After editing, run this script again."
    exit 0
fi

# 3. Build and start services
echo "[2/3] Building and starting services..."
cd "$DEPLOY_DIR"
docker compose down --remove-orphans 2>/dev/null || true
docker compose build --no-cache
docker compose up -d

# 4. Wait and verify
echo "[3/3] Waiting for services to start..."
sleep 10

if curl -s http://localhost:8081/health | grep -q '"ok"'; then
    echo ""
    echo "=========================================="
    echo "  Deployment successful!"
    echo ""
    echo "  Web UI:   http://localhost:8081"
    echo "  MCP:      http://localhost:8081/mcp"
    echo "  Health:   http://localhost:8081/health"
    echo "=========================================="
else
    echo ""
    echo "  Services may still be starting."
    echo "  Check logs with: docker compose logs -f"
fi
