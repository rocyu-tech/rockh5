#!/bin/bash
# ============================================================================
# RockGame Server Rebuild & Deploy Script
# Usage: bash deploy.sh [service]
#   service: account-node, admin-node, gate, or "all" (default)
# ============================================================================

set -e

DEPLOY_DIR="$(cd "$(dirname "$0")/deploy/docker" && pwd)"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"

SERVICE="${1:-all}"

echo "=========================================="
echo " RockGame Rebuild & Deploy"
echo " Service: $SERVICE"
echo "=========================================="

# Pull latest code
echo ""
echo "[1/3] Pulling latest code..."
cd "$PROJECT_DIR"
git pull origin main

case "$SERVICE" in
    account-node|node-account)
        echo ""
        echo "[2/3] Building account-node..."
        cd "$DEPLOY_DIR"
        docker-compose build node-account-0 node-account-1
        echo ""
        echo "[3/3] Restarting account-node..."
        docker-compose up -d node-account-0 node-account-1
        ;;
    admin-node|node-admin)
        echo ""
        echo "[2/3] Building admin-node..."
        cd "$DEPLOY_DIR"
        docker-compose build node-admin-0
        echo ""
        echo "[3/3] Restarting admin-node..."
        docker-compose up -d node-admin-0
        ;;
    gate)
        echo ""
        echo "[2/3] Building gate..."
        cd "$DEPLOY_DIR"
        docker-compose build gate-0 gate-1
        echo ""
        echo "[3/3] Restarting gate..."
        docker-compose up -d gate-0 gate-1
        ;;
    all)
        echo ""
        echo "[2/3] Building all services..."
        cd "$DEPLOY_DIR"
        docker-compose build
        echo ""
        echo "[3/3] Restarting all services..."
        docker-compose up -d
        ;;
    *)
        echo "Unknown service: $SERVICE"
        echo "Valid: account-node, admin-node, gate, all"
        exit 1
        ;;
esac

echo ""
echo "=========================================="
echo " Deploy complete! Service: $SERVICE"
echo "=========================================="

# Quick health check
sleep 3
echo ""
echo "Health check..."
curl -s http://localhost:8880/health 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "(Gate not accessible on localhost, check external IP)"

echo ""
echo "Verify account-node profile endpoint:"
TOKEN=$(curl -s http://localhost:8880/api/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"test@rockgame.com","password":"123456"}' 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])" 2>/dev/null)
if [ -n "$TOKEN" ]; then
    curl -s http://localhost:8880/api/v1/account/profile -H "Authorization: Bearer $TOKEN" 2>/dev/null | python3 -m json.tool 2>/dev/null
else
    echo "(Could not get token, check logs)"
fi
