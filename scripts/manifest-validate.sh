#!/usr/bin/env bash
set -euo pipefail

# Slack Manifest Validation Script
# Validates manifest.json against the JSON schema and checks for common issues

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
MANIFEST_FILE="${PROJECT_ROOT}/manifest.json"
SCHEMA_FILE="${PROJECT_ROOT}/specs/feat-slack-cli-integration/contracts/manifest-schema.json"

echo "Validating Slack manifest..."

# Check if manifest.json exists
if [[ ! -f "${MANIFEST_FILE}" ]]; then
    echo "ERROR: manifest.json not found at ${MANIFEST_FILE}"
    exit 1
fi

# Check if schema file exists
if [[ ! -f "${SCHEMA_FILE}" ]]; then
    echo "[WARN] Schema file not found, skipping schema validation"
    echo "   Expected: ${SCHEMA_FILE}"
else
    # Validate JSON syntax
    if ! jq empty "${MANIFEST_FILE}" 2>/dev/null; then
        echo "ERROR: manifest.json contains invalid JSON syntax"
        exit 1
    fi

    # Validate against JSON schema (if ajv-cli is available)
    if command -v ajv &> /dev/null; then
        echo "Validating against JSON schema..."
        ajv validate -s "${SCHEMA_FILE}" -d "${MANIFEST_FILE}" --strict=false || {
            echo "ERROR: Manifest validation failed"
            exit 1
        }
    else
        echo "[WARN] ajv-cli not installed, skipping schema validation"
        echo "   Install with: npm install -g ajv-cli"
    fi
fi

# Validate required fields
echo "Checking required fields..."

REQUIRED_FIELDS=(
    ".display_information.name"
    ".features.bot_user.display_name"
    ".oauth_config.scopes.bot"
    ".settings.socket_mode_enabled"
)

for field in "${REQUIRED_FIELDS[@]}"; do
    if ! jq -e "${field}" "${MANIFEST_FILE}" >/dev/null 2>&1; then
        echo "ERROR: Required field ${field} is missing"
        exit 1
    fi
done

# Validate Socket Mode vs HTTP Mode configuration
SOCKET_MODE=$(jq -r '.settings.socket_mode_enabled' "${MANIFEST_FILE}")

if [[ "${SOCKET_MODE}" == "true" ]]; then
    echo "Socket Mode enabled - checking configuration..."

    # Socket Mode should NOT have URLs in slash commands, interactivity, or events
    if jq -e '.features.slash_commands[]?.url' "${MANIFEST_FILE}" >/dev/null 2>&1; then
        echo "[WARN] Socket Mode enabled but slash commands have 'url' field"
        echo "   In Socket Mode, omit 'url' from slash commands"
    fi

    if jq -e '.settings.interactivity.request_url' "${MANIFEST_FILE}" >/dev/null 2>&1; then
        echo "[WARN] Socket Mode enabled but interactivity has 'request_url'"
        echo "   In Socket Mode, omit 'request_url' from interactivity"
    fi
else
    echo "HTTP Mode enabled - checking configuration..."

    # HTTP Mode SHOULD have URLs
    if ! jq -e '.settings.interactivity.request_url' "${MANIFEST_FILE}" >/dev/null 2>&1; then
        echo "[WARN] HTTP Mode enabled but interactivity missing 'request_url'"
    fi
fi

# Check for minimum required scopes
REQUIRED_SCOPES=("chat:write" "commands")
for scope in "${REQUIRED_SCOPES[@]}"; do
    if ! jq -e ".oauth_config.scopes.bot | index(\"${scope}\")" "${MANIFEST_FILE}" >/dev/null 2>&1; then
        echo "[WARN] Missing recommended scope: ${scope}"
    fi
done

echo "[OK] Manifest validation passed!"
echo ""
echo "Manifest Summary:"
echo "   App Name: $(jq -r '.display_information.name' "${MANIFEST_FILE}")"
echo "   Bot Display Name: $(jq -r '.features.bot_user.display_name' "${MANIFEST_FILE}")"
echo "   Socket Mode: ${SOCKET_MODE}"
echo "   Bot Scopes: $(jq -r '.oauth_config.scopes.bot | join(", ")' "${MANIFEST_FILE}")"
echo "   Slash Commands: $(jq -r '[.features.slash_commands[]?.command] | join(", ")' "${MANIFEST_FILE}")"
