#!/usr/bin/env bash

set -euo pipefail

#=============================================================================
# E2E Test Environment Setup Script
#=============================================================================
# Purpose: Single command to set up, run, and tear down E2E tests
# Usage: ./scripts/e2e-setup.sh
#
# Environment Variables:
#   E2E_BASE_PORT         - Base port for services (default: 9000)
#   E2E_SERVICE_TIMEOUT   - Service startup timeout in seconds (default: 60)
#   E2E_TEST_TIMEOUT      - Total test timeout in seconds (default: 300)
#   E2E_PRESERVE_ON_SUCCESS - Keep worktree on success (default: false)
#   E2E_VERBOSE           - Enable verbose logging (default: false)
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
readonly WORKTREE_BASE="${PROJECT_ROOT}/.worktrees"
readonly TIMESTAMP=$(date +%s)
readonly WORKTREE_DIR="${WORKTREE_BASE}/e2e-${TIMESTAMP}"

# Environment defaults
export E2E_BASE_PORT="${E2E_BASE_PORT:-9000}"
export E2E_BASE_PORT_ALERTMANAGER="${E2E_BASE_PORT_ALERTMANAGER:-9093}"
export E2E_MOCK_SLACK_PORT="${E2E_MOCK_SLACK_PORT:-9091}"
export E2E_MOCK_PAGERDUTY_PORT="${E2E_MOCK_PAGERDUTY_PORT:-9092}"
export E2E_ALERT_BRIDGE_PORT="${E2E_ALERT_BRIDGE_PORT:-9080}"
export E2E_SERVICE_TIMEOUT="${E2E_SERVICE_TIMEOUT:-60}"
export E2E_TEST_TIMEOUT="${E2E_TEST_TIMEOUT:-300}"
export E2E_PRESERVE_ON_SUCCESS="${E2E_PRESERVE_ON_SUCCESS:-false}"
export E2E_VERBOSE="${E2E_VERBOSE:-false}"

# Test execution state
TEST_FAILED=0
CLEANUP_DONE=0

#=============================================================================
# Logging Functions
#=============================================================================

log_info() {
    echo -e "${BLUE}[E2E]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[E2E]${NC} ✓ $*"
}

log_warning() {
    echo -e "${YELLOW}[E2E]${NC} ⚠ $*"
}

log_error() {
    echo -e "${RED}[E2E]${NC} ✗ $*" >&2
}

log_verbose() {
    if [[ "${E2E_VERBOSE}" == "true" ]]; then
        echo -e "${BLUE}[E2E][VERBOSE]${NC} $*"
    fi
}

#=============================================================================
# Prerequisite Validation
#=============================================================================

check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing_deps=0

    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        missing_deps=1
    else
        log_verbose "Docker: $(docker --version)"
    fi

    # Check Docker Compose
    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose v2 is not available"
        log_error "Install Docker Desktop or docker-compose-plugin"
        missing_deps=1
    else
        log_verbose "Docker Compose: $(docker compose version)"
    fi

    # Check Git
    if ! command -v git &> /dev/null; then
        log_error "Git is not installed"
        missing_deps=1
    else
        log_verbose "Git: $(git --version)"
    fi

    # Check Go
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        missing_deps=1
    else
        log_verbose "Go: $(go version)"
    fi

    # Check if Docker daemon is running
    if ! docker ps &> /dev/null; then
        log_error "Docker daemon is not running"
        log_error "Start Docker Desktop or dockerd service"
        missing_deps=1
    fi

    # Check port availability
    check_port_available "${E2E_BASE_PORT}" "Prometheus" || missing_deps=1
    check_port_available "${E2E_BASE_PORT_ALERTMANAGER}" "Alertmanager" || missing_deps=1
    check_port_available "${E2E_ALERT_BRIDGE_PORT}" "Alert-Bridge" || missing_deps=1
    check_port_available "${E2E_MOCK_SLACK_PORT}" "Mock Slack" || missing_deps=1
    check_port_available "${E2E_MOCK_PAGERDUTY_PORT}" "Mock PagerDuty" || missing_deps=1

    if [[ ${missing_deps} -eq 1 ]]; then
        log_error "Prerequisites check failed"
        exit 1
    fi

    log_success "All prerequisites met"
}

check_port_available() {
    local port=$1
    local service=$2

    if lsof -Pi :${port} -sTCP:LISTEN -t &>/dev/null; then
        log_error "Port ${port} (${service}) is already in use"
        log_error "Kill the process or set E2E_BASE_PORT to use different ports"
        return 1
    fi

    log_verbose "Port ${port} (${service}) is available"
    return 0
}

#=============================================================================
# Git Worktree Management
#=============================================================================

create_worktree() {
    log_info "Creating git worktree at ${WORKTREE_DIR}"

    # Create worktree base directory
    mkdir -p "${WORKTREE_BASE}"

    # Create worktree from current commit (detached HEAD to avoid branch conflicts)
    local current_commit
    current_commit=$(git -C "${PROJECT_ROOT}" rev-parse HEAD 2>/dev/null)

    if [[ -z "${current_commit}" ]]; then
        log_error "Failed to get current commit hash"
        exit 1
    fi

    local current_branch
    current_branch=$(git -C "${PROJECT_ROOT}" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "detached")

    log_verbose "Creating worktree from commit ${current_commit:0:8} (branch: ${current_branch})"

    if git -C "${PROJECT_ROOT}" worktree add --detach "${WORKTREE_DIR}" "${current_commit}" &>/dev/null; then
        log_success "Worktree created at ${WORKTREE_DIR}"
    else
        log_error "Failed to create worktree"
        log_error "Run 'git worktree list' to check existing worktrees"
        exit 1
    fi

    # Copy unstaged changes from working directory to worktree
    # This allows testing current work-in-progress code
    log_verbose "Syncing working directory changes to worktree..."

    # Get list of modified and untracked files (excluding .git, .worktrees, build artifacts)
    local changed_files
    changed_files=$(git -C "${PROJECT_ROOT}" diff --name-only HEAD 2>/dev/null || true)

    local untracked_files
    untracked_files=$(git -C "${PROJECT_ROOT}" ls-files --others --exclude-standard 2>/dev/null || true)

    # Combine changed and untracked files
    local all_files
    all_files=$(printf "%s\n%s" "${changed_files}" "${untracked_files}" | grep -v '^$' || true)

    if [[ -n "${all_files}" ]]; then
        log_verbose "Copying modified and untracked files to worktree..."
        echo "${all_files}" | while IFS= read -r file; do
            # Skip build artifacts and worktrees
            if [[ "${file}" == "alert-bridge" ]] || [[ "${file}" == .worktrees/* ]]; then
                continue
            fi

            if [[ -f "${PROJECT_ROOT}/${file}" ]]; then
                # Create parent directory in worktree if needed
                local target_dir
                target_dir=$(dirname "${WORKTREE_DIR}/${file}")
                mkdir -p "${target_dir}"

                # Copy file
                cp "${PROJECT_ROOT}/${file}" "${WORKTREE_DIR}/${file}"
                log_verbose "  Copied: ${file}"
            fi
        done
    fi

    # Copy E2E configurations to worktree
    log_verbose "Copying E2E configurations to worktree..."
    if [[ -d "${PROJECT_ROOT}/test/e2e-config" ]]; then
        cp -r "${PROJECT_ROOT}/test/e2e-config" "${WORKTREE_DIR}/test/"
        log_verbose "E2E configs copied"
    fi
}

cleanup_worktree() {
    if [[ ${CLEANUP_DONE} -eq 1 ]]; then
        return 0
    fi

    log_info "Cleaning up worktree..."

    # Stop and remove Docker containers
    cleanup_docker_services

    # Remove worktree
    if [[ -d "${WORKTREE_DIR}" ]]; then
        log_verbose "Removing worktree: ${WORKTREE_DIR}"

        # Remove worktree
        if git -C "${PROJECT_ROOT}" worktree remove "${WORKTREE_DIR}" --force &>/dev/null; then
            log_success "Worktree removed"
        else
            log_warning "Failed to remove worktree via git, trying manual removal"
            rm -rf "${WORKTREE_DIR}"
        fi
    fi

    CLEANUP_DONE=1
}

#=============================================================================
# Docker Service Management
#=============================================================================

start_docker_services() {
    log_info "Starting Docker services..."

    cd "${WORKTREE_DIR}/test"

    # Build and start services
    if docker compose -f e2e-docker-compose.yml up -d --build; then
        log_success "Docker services started"
    else
        log_error "Failed to start Docker services"
        exit 1
    fi
}

wait_for_services() {
    log_info "Waiting for services to be healthy (timeout: ${E2E_SERVICE_TIMEOUT}s)..."

    local services=("e2e-prometheus" "e2e-alertmanager" "e2e-mock-slack" "e2e-mock-pagerduty" "e2e-alert-bridge")
    local start_time=$(date +%s)
    local timeout=${E2E_SERVICE_TIMEOUT}

    for service in "${services[@]}"; do
        log_verbose "Waiting for ${service}..."

        local elapsed=0
        while true; do
            # Check if service is healthy
            local health_status
            health_status=$(docker inspect --format='{{.State.Health.Status}}' "${service}" 2>/dev/null || echo "unknown")

            if [[ "${health_status}" == "healthy" ]]; then
                log_success "${service} is healthy"
                break
            fi

            # Check timeout
            elapsed=$(($(date +%s) - start_time))
            if [[ ${elapsed} -ge ${timeout} ]]; then
                log_error "${service} did not become healthy within ${timeout}s"
                log_error "Current status: ${health_status}"
                docker logs "${service}" 2>&1 | tail -20
                return 1
            fi

            sleep 2
        done
    done

    log_success "All services are healthy"
}

cleanup_docker_services() {
    log_verbose "Stopping Docker services..."

    cd "${WORKTREE_DIR}/test" 2>/dev/null || cd "${PROJECT_ROOT}/test"

    # Stop and remove containers
    if docker compose -f e2e-docker-compose.yml down -v --remove-orphans &>/dev/null; then
        log_verbose "Docker services stopped"
    else
        log_warning "Failed to stop some Docker services gracefully"

        # Force remove E2E containers
        docker ps -a --filter "name=e2e-" --format "{{.Names}}" | while read -r container; do
            log_verbose "Force removing container: ${container}"
            docker rm -f "${container}" &>/dev/null || true
        done
    fi
}

#=============================================================================
# Test Execution
#=============================================================================

run_tests() {
    log_info "Running E2E test suite..."

    cd "${WORKTREE_DIR}"

    # Run Go tests
    local test_output
    local test_exit_code=0

    if test_output=$(go test -v -timeout "${E2E_TEST_TIMEOUT}s" ./test/e2e/... 2>&1); then
        log_success "Test suite passed!"
        echo "${test_output}"
    else
        test_exit_code=$?
        log_error "Test suite failed!"
        echo "${test_output}"
        TEST_FAILED=1
    fi

    return ${test_exit_code}
}

#=============================================================================
# Diagnostics Collection
#=============================================================================

collect_diagnostics() {
    log_info "Collecting diagnostics..."

    local diagnostics_dir="${WORKTREE_DIR}/diagnostics"
    mkdir -p "${diagnostics_dir}/services"

    # Collect container logs
    log_verbose "Collecting service logs..."
    local services=("e2e-prometheus" "e2e-alertmanager" "e2e-alert-bridge" "e2e-mock-slack" "e2e-mock-pagerduty")

    for service in "${services[@]}"; do
        if docker ps -a --format "{{.Names}}" | grep -q "^${service}$"; then
            docker logs "${service}" > "${diagnostics_dir}/services/${service}.log" 2>&1
            log_verbose "Collected logs for ${service}"
        fi
    done

    # Collect container states
    log_verbose "Collecting container states..."
    docker ps -a --filter "name=e2e-" --format "{{json .}}" > "${diagnostics_dir}/containers.json" 2>&1

    # Create test trace
    cat > "${diagnostics_dir}/test-trace.log" <<EOF
E2E Test Execution Trace
========================
Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
Worktree: ${WORKTREE_DIR}
Test Status: FAILED
Environment:
  E2E_BASE_PORT=${E2E_BASE_PORT}
  E2E_SERVICE_TIMEOUT=${E2E_SERVICE_TIMEOUT}
  E2E_TEST_TIMEOUT=${E2E_TEST_TIMEOUT}

Diagnostics Location: ${diagnostics_dir}
EOF

    log_success "Diagnostics collected at: ${diagnostics_dir}"
    echo ""
    echo "To view diagnostics:"
    echo "  Service logs:      cat ${diagnostics_dir}/services/*.log"
    echo "  Container states:  cat ${diagnostics_dir}/containers.json | jq"
    echo "  Test trace:        cat ${diagnostics_dir}/test-trace.log"
}

#=============================================================================
# Trap Handlers
#=============================================================================

cleanup_on_exit() {
    local exit_code=$?

    echo ""

    if [[ ${TEST_FAILED} -eq 1 ]]; then
        collect_diagnostics
    fi

    # Determine if we should preserve worktree
    local should_preserve=false
    if [[ ${TEST_FAILED} -eq 1 ]]; then
        should_preserve=true
    elif [[ "${E2E_PRESERVE_ON_SUCCESS}" == "true" ]]; then
        should_preserve=true
    fi

    if [[ "${should_preserve}" == "true" ]]; then
        log_warning "Preserving worktree for debugging: ${WORKTREE_DIR}"
        log_info "To clean up manually, run: ./scripts/e2e-cleanup.sh"
    else
        cleanup_worktree
        log_success "All resources cleaned up successfully"
    fi

    exit ${exit_code}
}

cleanup_on_interrupt() {
    echo ""
    log_warning "Interrupted by user"
    TEST_FAILED=1
    cleanup_on_exit
}

#=============================================================================
# Main Execution
#=============================================================================

main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║           Alert-Bridge E2E Test Suite                         ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""

    # Set up trap handlers
    trap cleanup_on_exit EXIT
    trap cleanup_on_interrupt INT TERM

    # Execution steps
    check_prerequisites
    create_worktree
    start_docker_services
    wait_for_services

    # Run tests (allow failure to trigger diagnostics)
    if run_tests; then
        echo ""
        log_success "✓ E2E test suite completed successfully!"
        TEST_FAILED=0
    else
        echo ""
        log_error "✗ E2E test suite failed"
        TEST_FAILED=1
    fi
}

# Run main function
main "$@"
