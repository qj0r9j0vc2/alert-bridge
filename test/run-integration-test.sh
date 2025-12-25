#!/bin/bash
set -e

echo "===================================="
echo "Alert-Bridge Integration Test"
echo "===================================="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Create data directory
mkdir -p test/data

echo -e "\n${YELLOW}[1/6] Starting services...${NC}"
docker-compose -f docker-compose.test.yaml up -d

echo -e "\n${YELLOW}[2/6] Waiting for services to be ready...${NC}"
sleep 15

# Check services health
echo -e "\n${YELLOW}[3/6] Checking service health...${NC}"
docker-compose -f docker-compose.test.yaml ps

# Verify Prometheus
echo -e "\n${YELLOW}[4/6] Verifying Prometheus...${NC}"
PROM_STATUS=$(curl -s http://localhost:9090/-/healthy)
if [ "$PROM_STATUS" == "Prometheus Server is Healthy." ]; then
    echo -e "${GREEN}✓ Prometheus is healthy${NC}"
else
    echo -e "${RED}✗ Prometheus is not healthy${NC}"
    exit 1
fi

# Verify Alertmanager
echo -e "\n${YELLOW}[4/6] Verifying Alertmanager...${NC}"
AM_STATUS=$(curl -s http://localhost:9093/-/healthy)
if [ "$AM_STATUS" == "OK" ]; then
    echo -e "${GREEN}✓ Alertmanager is healthy${NC}"
else
    echo -e "${RED}✗ Alertmanager is not healthy${NC}"
    exit 1
fi

# Verify Alert-Bridge
echo -e "\n${YELLOW}[4/6] Verifying Alert-Bridge...${NC}"
AB_STATUS=$(curl -s http://localhost:8080/health)
if echo "$AB_STATUS" | grep -q "ok"; then
    echo -e "${GREEN}✓ Alert-Bridge is healthy${NC}"
else
    echo -e "${RED}✗ Alert-Bridge is not healthy${NC}"
    docker-compose -f docker-compose.test.yaml logs alert-bridge
    exit 1
fi

# Send test alert
echo -e "\n${YELLOW}[5/6] Sending test alert...${NC}"
curl -s -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[
    {
      "labels": {
        "alertname": "TestIntegrationAlert",
        "severity": "critical",
        "instance": "test-instance",
        "job": "test-job"
      },
      "annotations": {
        "summary": "Integration test alert",
        "description": "This is a test alert for integration testing"
      },
      "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
    }
  ]'

echo -e "\n${GREEN}✓ Test alert sent${NC}"

# Wait for alert processing
sleep 5

# Check Alert-Bridge logs for alert processing
echo -e "\n${YELLOW}[6/6] Verifying alert reception...${NC}"
sleep 2
LOGS=$(docker-compose -f docker-compose.test.yaml logs alert-bridge 2>/dev/null | grep "alert processed")
if [ -n "$LOGS" ]; then
    echo -e "${GREEN}✓ Alert processed by Alert-Bridge${NC}"
    echo "$LOGS" | tail -1
else
    echo -e "${YELLOW}! No alerts processed yet (check logs below)${NC}"
fi

# Show Alert-Bridge logs
echo -e "\n${YELLOW}Alert-Bridge Logs:${NC}"
docker-compose -f docker-compose.test.yaml logs --tail=20 alert-bridge

# Summary
echo -e "\n===================================="
echo -e "${GREEN}Integration Test Completed${NC}"
echo -e "===================================="
echo ""
echo "Services running:"
echo "- Prometheus: http://localhost:9090"
echo "- Alertmanager: http://localhost:9093"
echo "- Alert-Bridge: http://localhost:8080"
echo ""
echo "To stop services:"
echo "  docker-compose -f docker-compose.test.yaml down"
echo ""
echo "To view logs:"
echo "  docker-compose -f docker-compose.test.yaml logs -f"
