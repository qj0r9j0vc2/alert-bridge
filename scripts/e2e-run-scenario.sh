#!/usr/bin/env bash

set -euo pipefail

#=============================================================================
# E2E Individual Scenario Runner
#=============================================================================
# Purpose: Run a single E2E test scenario for faster debugging
# Usage: ./scripts/e2e-run-scenario.sh <scenario-name>
#
# Scenarios:
#   alert-creation-slack       - Test alert delivery to Slack
#   alert-creation-pagerduty   - Test alert delivery to PagerDuty
#   alert-deduplication        - Test alert deduplication logic
#   alert-resolution           - Test alert resolution notifications
#   multiple-alerts-grouping   - Test multiple alerts grouping
#   different-severity-levels  - Test different severity handling
#=============================================================================

# Color codes for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Configuration
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Scenario mapping
declare -A SCENARIO_TESTS=(
    ["alert-creation-slack"]="TestAlertCreationSlack"
    ["alert-creation-pagerduty"]="TestAlertCreationPagerDuty"
    ["alert-deduplication"]="TestAlertDeduplication"
    ["alert-resolution"]="TestAlertResolution"
    ["multiple-alerts-grouping"]="TestMultipleAlertsGrouping"
    ["different-severity-levels"]="TestDifferentSeverityLevels"
)

#=============================================================================
# Logging Functions
#=============================================================================

log_info() {
    echo -e "${BLUE}[E2E-SCENARIO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[E2E-SCENARIO]${NC} ✓ $*"
}

log_error() {
    echo -e "${RED}[E2E-SCENARIO]${NC} ✗ $*" >&2
}

#=============================================================================
# Usage
#=============================================================================

show_usage() {
    cat <<EOF
Usage: $0 <scenario-name>

Available scenarios:
$(for scenario in "${!SCENARIO_TESTS[@]}"; do
    echo "  ${scenario}"
done | sort)

Environment Variables:
  E2E_BASE_PORT         - Base port for services (default: 9000)
  E2E_SERVICE_TIMEOUT   - Service startup timeout (default: 60)
  E2E_TEST_TIMEOUT      - Test timeout (default: 300)

Examples:
  $0 alert-creation-slack
  $0 alert-deduplication
  E2E_BASE_PORT=10000 $0 alert-resolution

EOF
}

#=============================================================================
# Main Execution
#=============================================================================

main() {
    if [[ $# -eq 0 ]]; then
        log_error "No scenario specified"
        echo ""
        show_usage
        exit 1
    fi

    local scenario="$1"

    if [[ ! -v SCENARIO_TESTS["${scenario}"] ]]; then
        log_error "Unknown scenario: ${scenario}"
        echo ""
        show_usage
        exit 1
    fi

    local test_name="${SCENARIO_TESTS[${scenario}]}"

    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║           Running E2E Scenario: ${scenario}"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""

    log_info "Test: ${test_name}"
    log_info "Using e2e-setup.sh to run single scenario..."
    echo ""

    # Export test filter
    export E2E_TEST_FILTER="${test_name}"

    # Run e2e-setup.sh with test filter
    cd "${PROJECT_ROOT}"

    # Modify the test run command to filter by test name
    # We'll do this by temporarily setting GOFLAGS
    export GOFLAGS="-run=^${test_name}$"

    if "${SCRIPT_DIR}/e2e-setup.sh"; then
        log_success "Scenario '${scenario}' passed!"
        exit 0
    else
        log_error "Scenario '${scenario}' failed!"
        exit 1
    fi
}

# Run main function
main "$@"
