#!/bin/bash
set -euo pipefail

# ============================================
# Azure Deployment Script - Container Apps (nginx + api + simulator)
# Run: chmod +x deploy-azure.sh && ./deploy-azure.sh
# Prereqs: az login, Docker, subscription
# ============================================

# ===== CONFIGURATION - EDIT THESE =====
RESOURCE_GROUP="rg-fault-detection-$(date +%s | tail -c 4)"
LOCATION="eastus2"
ACR_NAME="acrfault$(date +%s | tail -c 6)"        # Globally unique, lowercase, 5-50 chars
POSTGRES_SERVER="pgfault$(date +%s | tail -c 6)"  # Globally unique
POSTGRES_ADMIN="postgres"
POSTGRES_PASSWORD="$(openssl rand -base64 32 | tr -d '/+=' | head -c 25)"
CONTAINERAPP_ENV="ca-fault-detection"
NGINX_APP="nginx-fault"
API_APP="api-fault"
SIM_APP="sim-fault"
SEED_JOB="seed-job"
# ======================================

echo "============================================"
echo "Azure Container Apps Deployment"
echo "Resource Group: $RESOURCE_GROUP"
echo "Location: $LOCATION"
echo "ACR: $ACR_NAME"
echo "PostgreSQL: $POSTGRES_SERVER"
echo "============================================"

# 1. Login check
echo "[1/11] Checking Azure login..."
az account show >/dev/null 2>&1 || { echo "ERROR: Run 'az login' first"; exit 1; }
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
echo "Subscription: $SUBSCRIPTION_ID"

# 2. Create Resource Group
echo "[2/11] Creating resource group..."
az group create --name "$RESOURCE_GROUP" --location "$LOCATION" -o none

# 3. Create ACR
echo "[3/11] Creating Azure Container Registry..."
az acr create \
  --name "$ACR_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --sku Basic \
  --admin-enabled true \
  -o none

ACR_LOGIN_SERVER=$(az acr show --name "$ACR_NAME" --query loginServer -o tsv)
echo "ACR: $ACR_LOGIN_SERVER"

# 4. Create PostgreSQL
echo "[4/11] Creating PostgreSQL Flexible Server..."
az postgres flexible-server create \
  --name "$POSTGRES_SERVER" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  --admin-user "$POSTGRES_ADMIN" \
  --admin-password "$POSTGRES_PASSWORD" \
  --sku-name Standard_B1ms \
  --tier Burstable \
  --version 16 \
  --public-access 0.0.0.0 \
  --storage-size 32 \
  -o none

POSTGRES_FQDN="${POSTGRES_SERVER}.postgres.database.azure.com"
DATABASE_URL="postgres://${POSTGRES_ADMIN}:${POSTGRES_PASSWORD}@${POSTGRES_FQDN}:5432/fault_detection?sslmode=require"
echo "PostgreSQL: $POSTGRES_FQDN"

# 5. Create Container Apps Environment
echo "[5/11] Creating Container Apps Environment..."
az containerapp env create \
  --name "$CONTAINERAPP_ENV" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  -o none

# 6. Build & Push Images
echo "[6/11] Building and pushing Docker images..."
az acr login --name "$ACR_NAME"

docker buildx create --name multiarch --use 2>/dev/null || docker buildx use multiarch
echo "  Building api..."
docker buildx build --platform linux/amd64 --target api-final -t "${ACR_LOGIN_SERVER}/api:latest" --push . >/dev/null
echo "  Building simulator..."
docker buildx build --platform linux/amd64 --target simulator-final -t "${ACR_LOGIN_SERVER}/simulator:latest" --push . >/dev/null
echo "  Building seed..."
docker buildx build --platform linux/amd64 --target seed-final -t "${ACR_LOGIN_SERVER}/seed:latest" --push . >/dev/null
echo "  Building nginx (with frontend)..."
# Build nginx with frontend included
docker buildx build --platform linux/amd64 -f - -t "${ACR_LOGIN_SERVER}/nginx:latest" --push . <<'DOCKER_EOF'
FROM nginx:alpine
COPY nginx.conf /etc/nginx/nginx.conf:ro
COPY frontend/dist /usr/share/nginx/html:ro
DOCKER_EOF

# 7. Deploy API (internal ingress)
echo "[7/11] Deploying API Container App..."
az containerapp create \
  --name "$API_APP" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$CONTAINERAPP_ENV" \
  --image "${ACR_LOGIN_SERVER}/api:latest" \
  --target-port 8080 \
  --ingress internal \
  --registry-server "$ACR_LOGIN_SERVER" \
  --env-vars \
    DATABASE_URL="$DATABASE_URL" \
    SIMULATOR_URL="http://${SIM_APP}:8081" \
    API_PORT="8080" \
  -o none

# 8. Deploy Simulator (internal ingress)
echo "[8/11] Deploying Simulator Container App..."
az containerapp create \
  --name "$SIM_APP" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$CONTAINERAPP_ENV" \
  --image "${ACR_LOGIN_SERVER}/simulator:latest" \
  --target-port 8081 \
  --ingress internal \
  --registry-server "$ACR_LOGIN_SERVER" \
  --env-vars \
    DATABASE_URL="$DATABASE_URL" \
    API_URL="http://${API_APP}:8080" \
    SIM_PORT="8081" \
    CLOCK_MULTIPLIER="30" \
  -o none

# 9. Deploy Nginx (EXTERNAL ingress - single public URL)
echo "[9/11] Deploying Nginx Container App (public entry point)..."
az containerapp create \
  --name "$NGINX_APP" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$CONTAINERAPP_ENV" \
  --image "${ACR_LOGIN_SERVER}/nginx:latest" \
  --target-port 80 \
  --ingress external \
  --registry-server "$ACR_LOGIN_SERVER" \
  -o none

NGINX_FQDN=$(az containerapp show --name "$NGINX_APP" --resource-group "$RESOURCE_GROUP" --query properties.configuration.ingress.fqdn -o tsv)
echo "  Nginx FQDN: $NGINX_FQDN"

# 10. Run Seed Job
echo "[10/11] Running database seed job..."
az containerapp job create \
  --name "$SEED_JOB" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$CONTAINERAPP_ENV" \
  --image "${ACR_LOGIN_SERVER}/seed:latest" \
  --registry-server "$ACR_LOGIN_SERVER" \
  --command "./generator" --args "--seed-db" "--export-csv" "--pole-count" "3000" \
  --env-vars DATABASE_URL="$DATABASE_URL" \
  -o none

az containerapp job start --name "$SEED_JOB" --resource-group "$RESOURCE_GROUP" -o none
echo "  Seed job started."

# 11. Wait for seed and verify
echo "[11/11] Waiting for seed to complete (60s)..."
sleep 60

echo ""
echo "============================================"
echo "✅ DEPLOYMENT COMPLETE!"
echo "============================================"
echo ""
echo "📍 PUBLIC URL (Azure-provided domain):"
echo "   https://${NGINX_FQDN}"
echo ""
echo "   This single URL serves:"
echo "   - Frontend dashboard"
echo "   - API at /api/*"
echo "   - Simulator at /sim/*, /clock, /scheduled-outages"
echo "   - Ingest at /ingest"
echo "   - Health at /healthz"
echo ""
echo "🔐 Database Credentials (save these!):"
echo "   Server:   ${POSTGRES_FQDN}"
echo "   Database: fault_detection"
echo "   User:     ${POSTGRES_ADMIN}"
echo "   Password: ${POSTGRES_PASSWORD}"
echo ""
echo "🔗 Connection String:"
echo "   ${DATABASE_URL}"
echo ""
echo "📋 Verification (run after ~2 min):"
echo "   curl https://${NGINX_FQDN}/healthz"
echo "   curl https://${NGINX_FQDN}/api/tickets"
echo "   curl https://${NGINX_FQDN}/sim/topology/tree"
echo "   Open: https://${NGINX_FQDN}"
echo ""
echo "💰 Estimated Monthly Cost: ~$15-25 (Hobby tier)"
echo "============================================"

# Save credentials
cat > azure-deployment-output.txt <<EOF
DEPLOYMENT COMPLETE - $(date)

PUBLIC URL:
https://${NGINX_FQDN}

DATABASE:
- Server: ${POSTGRES_FQDN}
- Database: fault_detection
- User: ${POSTGRES_ADMIN}
- Password: ${POSTGRES_PASSWORD}
- Connection String: ${DATABASE_URL}

RESOURCE GROUP: ${RESOURCE_GROUP}
ACR: ${ACR_LOGIN_SERVER}
CONTAINER APP ENV: ${CONTAINERAPP_ENV}
APPS: ${NGINX_APP}, ${API_APP}, ${SIM_APP}
SEED JOB: ${SEED_JOB}
EOF

echo "Credentials saved to azure-deployment-output.txt"