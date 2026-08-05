#!/bin/bash
set -euo pipefail

# ============================================
# Simple VM + Docker Compose - Just works
# Uses a commonly available size/region
# ============================================

RESOURCE_GROUP="rg-fault-detection"
LOCATION="centralindia"
VM_NAME="vm-fault-detection"
VM_SIZE="Standard_D2s_v3"  # Usually available in India
ADMIN_USER="azureuser"
DNS_LABEL="fault-detection-$(date +%s | tail -c 6)"

echo "============================================"
echo "Simple VM Deployment (centralindia, D2s_v3)"
echo "============================================"

# Clean up
echo "[1/6] Cleaning up..."
az group delete --name "$RESOURCE_GROUP" --yes --no-wait 2>/dev/null || true
sleep 3

# Create resource group
echo "[2/6] Creating resource group..."
az group create --name "$RESOURCE_GROUP" --location "$LOCATION" -o none

# Create VM
echo "[3/6] Creating VM..."
cat > cloud-init.yml <<'EOF'
#cloud-config
package_update: true
packages:
  - docker.io
  - docker-compose-v2
  - git
  - postgresql-client
runcmd:
  - systemctl enable docker
  - systemctl start docker
  - usermod -aG docker azureuser
  - mkdir -p /opt/fault-detection
  - chown azureuser:azureuser /opt/fault-detection
EOF

az vm create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME" \
  --image Ubuntu2204 \
  --size "$VM_SIZE" \
  --admin-username "$ADMIN_USER" \
  --generate-ssh-keys \
  --public-ip-sku Standard \
  --public-ip-address-dns-name "$DNS_LABEL" \
  --custom-data cloud-init.yml \
  --storage-sku Standard_LRS \
  --no-wait

echo "VM creation started (async). Waiting 120s for VM to be ready..."
sleep 120

# Get public IP
PUBLIC_IP=$(az vm show -d -g "$RESOURCE_GROUP" -n "$VM_NAME" --query publicIps -o tsv)
FQDN="${DNS_LABEL}.${LOCATION}.cloudapp.azure.com"
echo "  VM Public IP: $PUBLIC_IP"
echo "  VM FQDN: $FQDN"

# Deploy project
echo "[4/6] Deploying project..."
az vm run-command invoke \
  --resource-group "$RESOURCE_GROUP" \
  --name "$VM_NAME" \
  --command-id RunShellScript \
  --scripts "
    cd /opt/fault-detection &&
    git clone https://github.com/vedanta00011/Power-Distribution-Fault-Detection-System.git . 2>/dev/null || git pull &&
    sed -i 's/8080:80/80:80/' docker-compose.yml &&
    docker compose up --build -d
  " \
  --output none

# Open port 80
echo "[5/6] Opening port 80..."
az vm open-port --resource-group "$RESOURCE_GROUP" --name "$VM_NAME" --port 80 --priority 100 -o none

echo ""
echo "============================================"
echo "✅ DEPLOYMENT STARTED!"
echo "============================================"
echo ""
echo "📍 PUBLIC URL (ready in ~3-5 min):"
echo "   http://${FQDN}"
echo "   http://${PUBLIC_IP}"
echo ""
echo "🔐 SSH Access:"
echo "   ssh ${ADMIN_USER}@${FQDN}"
echo ""
echo "📋 Check status on VM:"
echo "   ssh ${ADMIN_USER}@${FQDN} 'cd /opt/fault-detection && docker compose logs -f'"
echo ""
echo "💰 Cost: ~$20-30/month (D2s_v3)"
echo "============================================"

cat > deployment-info.txt <<EOF
PUBLIC URL: http://${FQDN}
SSH: ssh ${ADMIN_USER}@${FQDN}
VM: ${VM_NAME}
RESOURCE GROUP: ${RESOURCE_GROUP}
EOF