#!/usr/bin/env bash
set -euo pipefail

# Slack CLI Integration Quick Start Script
# Automates the fork-to-first-connection workflow in <10 minutes

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "Alert Bridge - Slack CLI Integration Quick Start"
echo "=================================================="
echo ""

# Step 1: Validate Prerequisites
echo "Step 1/6: Validating prerequisites..."

# Check Slack CLI
if ! command -v slack &> /dev/null; then
    echo "ERROR: Slack CLI not found"
    echo ""
    echo "Please install Slack CLI:"
    echo "  macOS:   brew install slack-cli/tap/slack-cli"
    echo "  Linux:   curl -fsSL https://downloads.slack-edge.com/slack-cli/install.sh | bash"
    echo "  Windows: See https://api.slack.com/automation/cli/install"
    exit 1
fi

SLACK_VERSION=$(slack version 2>&1 | head -n1 || echo "unknown")
echo "  [OK] Slack CLI installed: ${SLACK_VERSION}"

# Check Docker (optional but recommended)
if command -v docker &> /dev/null; then
    DOCKER_VERSION=$(docker --version | cut -d' ' -f3 | tr -d ',')
    echo "  [OK] Docker installed: ${DOCKER_VERSION}"
else
    echo "  [WARN] Docker not found (optional, but recommended for deployment)"
fi

# Check Go
if ! command -v go &> /dev/null; then
    echo "ERROR: Go not found"
    echo "Please install Go 1.24.2 or later: https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo "  [OK] Go installed: ${GO_VERSION}"

# Verify Go version is 1.24.2+
GO_MINOR=$(go version | sed -E 's/.*go1\.([0-9]+).*/\1/')
if [[ ${GO_MINOR} -lt 24 ]]; then
    echo "  [WARN] Go 1.24.2+ recommended, found ${GO_VERSION}"
fi

echo ""

# Step 2: Slack Login
echo "Step 2/6: Logging into Slack workspace..."
echo ""
echo "You'll be redirected to your browser to authorize the Slack CLI."
echo "This is a one-time setup."
echo ""
read -p "Press Enter to continue..."

if slack login; then
    echo "  [OK] Logged into Slack workspace"
else
    echo "ERROR: Slack login failed"
    exit 1
fi

echo ""

# Step 3: Create Slack App
echo "Step 3/6: Creating Slack app from manifest..."
echo ""

MANIFEST_PATH="${PROJECT_ROOT}/manifest.json"

if [[ ! -f "${MANIFEST_PATH}" ]]; then
    echo "ERROR: manifest.json not found at ${MANIFEST_PATH}"
    exit 1
fi

echo "Using manifest: ${MANIFEST_PATH}"
echo ""
echo "You'll be prompted to:"
echo "  1. Select your Slack workspace"
echo "  2. Confirm app creation"
echo "  3. Install the app to your workspace"
echo ""
read -p "Press Enter to create app..."

# Create app and capture output
if slack create --manifest "${MANIFEST_PATH}"; then
    echo "  [OK] Slack app created successfully!"
else
    echo "ERROR: Failed to create Slack app"
    echo ""
    echo "Troubleshooting:"
    echo "  - Verify you have admin permissions in the workspace"
    echo "  - Check manifest.json is valid: ./scripts/manifest-validate.sh"
    echo "  - Try: slack manifest validate --manifest manifest.json"
    exit 1
fi

echo ""

# Step 4: Install App
echo "Step 4/6: Installing app to workspace..."
echo ""

if slack install; then
    echo "  [OK] App installed to workspace"
else
    echo "  [WARN] You may need to manually install the app from Slack's app management page"
fi

echo ""

# Step 5: Get Tokens
echo "Step 5/6: Collecting Slack tokens..."
echo ""
echo "To complete the setup, you need two tokens from Slack:"
echo ""
echo "1. Bot User OAuth Token (xoxb-...)"
echo "   - Go to: https://api.slack.com/apps"
echo "   - Select your 'Alert Bridge' app"
echo "   - Navigate to: OAuth & Permissions"
echo "   - Copy the 'Bot User OAuth Token'"
echo ""
echo "2. App-Level Token (xapp-...)"
echo "   - In the same app page"
echo "   - Navigate to: Basic Information -> App-Level Tokens"
echo "   - Generate a token with 'connections:write' scope if not exists"
echo "   - Copy the token"
echo ""
echo "3. Channel ID (C...)"
echo "   - Open Slack in browser"
echo "   - Navigate to the channel where you want alerts"
echo "   - Copy the channel ID from URL: https://app.slack.com/client/T.../C0123456"
echo ""

# Prompt for tokens
read -p "Bot User OAuth Token (xoxb-...): " SLACK_BOT_TOKEN
read -p "App-Level Token (xapp-...): " SLACK_APP_TOKEN
read -p "Channel ID (C...): " SLACK_CHANNEL_ID

# Validate token formats
if [[ ! "${SLACK_BOT_TOKEN}" =~ ^xoxb- ]]; then
    echo "  [WARN] Bot token should start with 'xoxb-'"
fi

if [[ ! "${SLACK_APP_TOKEN}" =~ ^xapp- ]]; then
    echo "  [WARN] App token should start with 'xapp-'"
fi

if [[ ! "${SLACK_CHANNEL_ID}" =~ ^C[A-Z0-9]+ ]]; then
    echo "  [WARN] Channel ID should start with 'C' followed by alphanumeric characters"
fi

echo ""

# Step 6: Create config.yaml
echo "Step 6/6: Creating configuration file..."
echo ""

CONFIG_PATH="${PROJECT_ROOT}/config/config.yaml"
EXAMPLE_CONFIG="${PROJECT_ROOT}/config/config.example.yaml"

if [[ -f "${CONFIG_PATH}" ]]; then
    echo "  [WARN] config.yaml already exists"
    read -p "Overwrite? (y/N): " OVERWRITE
    if [[ ! "${OVERWRITE}" =~ ^[Yy]$ ]]; then
        echo "Skipping config creation. Please update config.yaml manually."
        exit 0
    fi
fi

# Create config from example with token substitution
if [[ ! -f "${EXAMPLE_CONFIG}" ]]; then
    echo "ERROR: config.example.yaml not found"
    exit 1
fi

# Use sed to replace environment variables in the example config
cat "${EXAMPLE_CONFIG}" | \
    sed "s|\${SLACK_BOT_TOKEN}|${SLACK_BOT_TOKEN}|g" | \
    sed "s|\${SLACK_CHANNEL_ID}|${SLACK_CHANNEL_ID}|g" | \
    sed "s|\${SLACK_SOCKET_MODE_APP_TOKEN}|${SLACK_APP_TOKEN}|g" | \
    sed "s|enabled: false.*# Set to true for local dev|enabled: true                                 # Socket Mode enabled for local dev|g" \
    > "${CONFIG_PATH}"

echo "  [OK] Created ${CONFIG_PATH}"

echo ""
echo "============================================================"
echo "Setup Complete!"
echo "============================================================"
echo ""
echo "Next steps:"
echo ""
echo "1. Start Alert Bridge:"
echo "   go run cmd/alert-bridge/main.go"
echo ""
echo "   OR with Docker:"
echo "   docker-compose up --build"
echo ""
echo "2. Look for this log message:"
echo '   {"level":"info","msg":"Connected to Slack via Socket Mode!","connection_id":"..."}'
echo ""
echo "3. Test the slash command in Slack:"
echo "   /alert-status"
echo ""
echo "4. See alerts appear in channel: #${SLACK_CHANNEL_ID}"
echo ""
echo "Full documentation: specs/feat-slack-cli-integration/quickstart.md"
echo ""
