#!/bin/bash
#
# Test script for PagerDuty webhook signature validation
# Based on quickstart.md Appendix C
#
# Usage:
#   ./test-pagerduty-webhook-signature.sh <webhook-secret> <alert-bridge-url>
#
# Example:
#   ./test-pagerduty-webhook-signature.sh "whsec_test123" "http://localhost:8080"
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check arguments
if [ $# -lt 2 ]; then
    echo "Usage: $0 <webhook-secret> <alert-bridge-url>"
    echo "Example: $0 'whsec_test123' 'http://localhost:8080'"
    exit 1
fi

WEBHOOK_SECRET="$1"
BASE_URL="$2"
ENDPOINT="${BASE_URL}/webhook/pagerduty"

echo "=============================================="
echo "PagerDuty Webhook Signature Validation Tests"
echo "=============================================="
echo "Endpoint: $ENDPOINT"
echo "Secret: ${WEBHOOK_SECRET:0:10}..."
echo ""

# Test payload
PAYLOAD='{
  "messages": [{
    "id": "test-message-001",
    "event": {
      "id": "test-event-001",
      "event_type": "incident.acknowledged",
      "resource_type": "incident",
      "occurred_at": "2025-12-26T10:00:00Z",
      "agent": {
        "id": "PTEST123",
        "type": "user_reference",
        "summary": "Test User",
        "email": "test@example.com",
        "name": "Test User"
      },
      "data": {
        "id": "PDTEST",
        "type": "incident",
        "incident_key": "test-fingerprint-12345",
        "status": "acknowledged"
      }
    },
    "created_on": "2025-12-26T10:00:00Z"
  }]
}'

# Function to generate HMAC signature
generate_signature() {
    local payload="$1"
    local secret="$2"

    # Compute HMAC-SHA256
    local hmac=$(echo -n "$payload" | openssl dgst -sha256 -hmac "$secret" -binary | xxd -p -c 256)
    echo "v1=${hmac}"
}

# Function to test webhook with signature
test_webhook() {
    local test_name="$1"
    local signature="$2"
    local expected_code="$3"

    echo -n "Test: $test_name... "

    # Build curl command
    local curl_cmd="curl -s -w '\n%{http_code}' -X POST '$ENDPOINT' \
        -H 'Content-Type: application/json'"

    if [ -n "$signature" ]; then
        curl_cmd="$curl_cmd -H 'X-PagerDuty-Signature: $signature'"
    fi

    curl_cmd="$curl_cmd -d '$PAYLOAD'"

    # Execute request
    local response=$(eval "$curl_cmd")
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | sed '$d')

    # Validate response
    if [ "$http_code" = "$expected_code" ]; then
        echo -e "${GREEN}PASS${NC} (HTTP $http_code)"
        if [ -n "$body" ] && [ "$body" != "missing signature" ] && [ "$body" != "invalid signature" ]; then
            echo "  Response: $body"
        fi
    else
        echo -e "${RED}FAIL${NC} (Expected HTTP $expected_code, got $http_code)"
        echo "  Response: $body"
        return 1
    fi
}

# Generate valid signature
VALID_SIGNATURE=$(generate_signature "$PAYLOAD" "$WEBHOOK_SECRET")

echo "---------------------------------------------"
echo "Test Suite 1: Signature Validation"
echo "---------------------------------------------"

# Test 1: Valid signature
test_webhook "Valid signature" "$VALID_SIGNATURE" "200"

# Test 2: Invalid signature
test_webhook "Invalid signature" "v1=invalid0000000000000000000000000000000000000000000000000000000000000000" "401"

# Test 3: Missing signature header
test_webhook "Missing signature" "" "401"

# Test 4: Malformed signature (no v1= prefix)
test_webhook "Malformed signature (no prefix)" "abc123" "401"

# Test 5: Wrong version prefix
test_webhook "Wrong version (v2)" "v2=abc123def456" "401"

echo ""
echo "---------------------------------------------"
echo "Test Suite 2: Log Verification"
echo "---------------------------------------------"
echo "Check alert-bridge logs for the following:"
echo "  - ${GREEN}✓${NC} Successful validation: 'pagerduty webhook signature validated' with remote_addr"
echo "  - ${YELLOW}!${NC} Failed validation: 'pagerduty webhook signature validation failed' with reason"
echo ""
echo "Expected log entries:"
echo '  {"level":"info","msg":"pagerduty webhook signature validated","remote_addr":"...","user_agent":"..."}'
echo '  {"level":"warn","msg":"pagerduty webhook signature validation failed","reason":"invalid_signature","remote_addr":"..."}'
echo '  {"level":"warn","msg":"pagerduty webhook signature validation failed","reason":"missing_signature","remote_addr":"..."}'
echo ""

echo "=============================================="
echo "All tests completed!"
echo "=============================================="
