#!/bin/bash
set -euo pipefail

# ============================================
# Azure Container Instances - Simplest & Cheapest
# Single container group with docker-compose equivalent
# Cost: ~$10-15/month
# ============================================

RESOURCE_GROUP="rg-fault-detection"
LOCATION="centralindia"
ACI_NAME="aci-fault-detection"
DNS_LABEL="fault-detection-$(date +%s | tail -c 6)"

echo "============================================"
echo "Azure Container Instances Deployment"
echo "Location: $LOCATION"
echo "DNS: $DNS_LABEL.$LOCATION.azurecontainer.io"
echo "============================================"

# Clean up
echo "[1/5] Cleaning up..."
az group delete --name "$RESOURCE_GROUP" --yes --no-wait 2>/dev/null || true
sleep 3

# Create resource group
echo "[2/5] Creating resource group..."
az group create --name "$RESOURCE_GROUP" --location "$LOCATION" -o none

# Create ACR
echo "[3/5] Creating ACR..."
ACR_NAME="acrfault$(date +%s | tail -c 6)"
az acr create --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" --sku Basic --admin-enabled true -o none
ACR_LOGIN_SERVER=$(az acr show --name "$ACR_NAME" --query loginServer -o tsv)
az acr login --name "$ACR_NAME"

# Build and push single combined image (nginx + api + simulator)
echo "[4/5] Building combined image..."
docker buildx create --name multiarch --use 2>/dev/null || docker buildx use multiarch

# Build a single image that runs everything via supervisord
cat > Dockerfile.aci <<'EOF'
FROM ubuntu:22.04

# Install supervisor, nginx, postgresql-client, curl
RUN apt-get update && apt-get install -y \
    supervisor \
    nginx \
    postgresql-client \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Go binaries (copy from build stages)
COPY --from=ghcr.io/vedanta00011/power-fault-detector/api:latest /api /usr/local/bin/api
COPY --from=ghcr.io/vedanta00011/power-fault-detector/simulator:latest /simulator /usr/local/bin/simulator
COPY --from=ghcr.io/vedanta00011/power-fault-detector/generator:latest /generator /usr/local/bin/generator

# Copy nginx config and frontend
COPY nginx.conf /etc/nginx/nginx.conf
COPY frontend/dist /usr/share/nginx/html

# Copy schema
COPY internal/db/schema.sql /app/internal/db/schema.sql
RUN mkdir -p /app/data

# Supervisor config
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

EXPOSE 80

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
EOF

cat > supervisord.conf <<'EOF'
[supervisord]
nodaemon=true
logfile=/var/log/supervisord.log
pidfile=/var/run/supervisord.pid

[program:postgres-wait]
command=bash -c 'until pg_isready -h $POSTGRES_HOST -U postgres; do sleep 2; done'
autostart=true
autorestart=false
stdout_logfile=/var/log/postgres-wait.log

[program:seed]
command=bash -c 'sleep 5 && /usr/local/bin/generator --seed-db --export-csv --pole-count 3000'
autostart=true
autorestart=false
stdout_logfile=/var/log/seed.log

[program:api]
command=/usr/local/bin/api
autostart=true
autorestart=true
stdout_logfile=/var/log/api.log
stderr_logfile=/var/log/api.err
environment=DATABASE_URL="postgres://postgres:postgres@$POSTGRES_HOST:5432/fault_detection?sslmode=disable",SIMULATOR_URL="http://localhost:8081",API_PORT="8080"

[program:simulator]
command=/usr/local/bin/simulator
autostart=true
autorestart=true
stdout_logfile=/var/log/simulator.log
stderr_logfile=/var/log/simulator.err
environment=DATABASE_URL="postgres://postgres:postgres@$POSTGRES_HOST:5432/fault_detection?sslmode=disable",API_URL="http://localhost:8080",SIM_PORT="8081",CLOCK_MULTIPLIER="30"

[program:nginx]
command=nginx -g "daemon off;"
autostart=true
autorestart=true
stdout_logfile=/var/log/nginx.log
stderr_logfile=/var/log/nginx.err
EOF

# Since we can't use multi-stage from GHCR easily, let's just build locally
echo "Building all images..."
docker buildx build --platform linux/amd64 --target api-final -t "${ACR_LOGIN_SERVER}/api:latest" --push . >/dev/null
docker buildx build --platform linux/amd64 --target simulator-final -t "${ACR_LOGIN_SERVER}/simulator:latest" --push . >/dev/null
docker buildx build --platform linux/amd64 --target seed-final -t "${ACR_LOGIN_SERVER}/seed:latest" --push . >/dev/null

# Build the combined ACI image
docker buildx build --platform linux/amd64 -f Dockerfile.aci -t "${ACR_LOGIN_SERVER}/aci-combined:latest" --push . >/dev/null

# Create PostgreSQL (using Azure Database for PostgreSQL - but that's slow)
# Instead, run postgres in the same container group
echo "[5/5] Creating Container Instance..."

az container create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$ACI_NAME" \
  --image "${ACR_LOGIN_SERVER}/aci-combined:latest" \
  --registry-login-server "$ACR_LOGIN_SERVER" \
  --registry-username "$ACR_NAME" \
  --registry-password "$(az acr credential show --name "$ACR_NAME" --query passwords[0].value -o tsv)" \
  --dns-name-label "$DNS_LABEL" \
  --ports 80 5432 \
  --environment-variables \
    POSTGRES_HOST=localhost \
    POSTGRES_DB=fault_detection \
    POSTGRES_USER=postgres \
    POSTGRES_PASSWORD=postgres \
  --cpu 2 \
  --memory 4 \
  --location "$LOCATION" \
  -o none

FQDN="${DNS_LABEL}.${LOCATION}.azurecontainer.io"
echo ""
echo "============================================"
echo "✅ DEPLOYMENT COMPLETE!"
echo "============================================"
echo ""
echo "📍 PUBLIC URL:"
echo "   http://${FQDN}"
echo ""
echo "⏳ Wait 2-3 minutes for database initialization and seeding"
echo ""
echo "📋 Check status:"
echo "   az container show --name $ACI_NAME --resource-group $RESOURCE_GROUP --query instanceView.state -o tsv"
echo "   az container logs --name $ACI_NAME --resource-group $RESOURCE_GROUP"
echo ""
echo "💰 Cost: ~$15-20/month (2 vCPU, 4GB)"
echo "============================================"

cat > deployment-info.txt <<EOF
PUBLIC URL: http://${FQDN}
ACI: ${ACI_NAME}
RESOURCE GROUP: ${RESOURCE_GROUP}
ACR: ${ACR_LOGIN_SERVER}
EOF