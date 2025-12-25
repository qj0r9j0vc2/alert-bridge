#!/usr/bin/env bash

set -euo pipefail

#=============================================================================
# E2E Test Environment Cleanup Script
#=============================================================================
# Purpose: Manual cleanup of E2E test resources when automatic cleanup fails
# Usage: ./scripts/e2e-cleanup.sh [--all | --worktrees | --containers]
#
# Options:
#   --all         Clean up everything (default)
#   --worktrees   Only clean up worktrees
#   --containers  Only clean up Docker containers
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

# Cleanup options
CLEANUP_WORKTREES=true
CLEANUP_CONTAINERS=true

#=============================================================================
# Logging Functions
#=============================================================================

log_info() {
    echo -e "${BLUE}[CLEANUP]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[CLEANUP]${NC} ✓ $*"
}

log_warning() {
    echo -e "${YELLOW}[CLEANUP]${NC} ⚠ $*"
}

log_error() {
    echo -e "${RED}[CLEANUP]${NC} ✗ $*" >&2
}

#=============================================================================
# Cleanup Functions
#=============================================================================

cleanup_containers() {
    log_info "Stopping E2E Docker containers..."

    local containers_stopped=0
    local containers_removed=0

    # Find all E2E containers
    local e2e_containers
    e2e_containers=$(docker ps -a --filter "name=e2e-" --format "{{.Names}}" 2>/dev/null || true)

    if [[ -z "${e2e_containers}" ]]; then
        log_info "No E2E containers found"
        return 0
    fi

    # Stop containers
    while IFS= read -r container; do
        if [[ -n "${container}" ]]; then
            log_info "Stopping container: ${container}"
            if docker stop "${container}" &>/dev/null; then
                ((containers_stopped++))
            else
                log_warning "Failed to stop ${container}"
            fi
        fi
    done <<< "${e2e_containers}"

    # Remove containers
    while IFS= read -r container; do
        if [[ -n "${container}" ]]; then
            log_info "Removing container: ${container}"
            if docker rm -f "${container}" &>/dev/null; then
                ((containers_removed++))
            else
                log_warning "Failed to remove ${container}"
            fi
        fi
    done <<< "${e2e_containers}"

    # Clean up E2E network
    if docker network ls --format "{{.Name}}" | grep -q "^e2e-test-network$"; then
        log_info "Removing E2E network..."
        docker network rm e2e-test-network &>/dev/null || log_warning "Failed to remove E2E network"
    fi

    log_success "Stopped ${containers_stopped} containers, removed ${containers_removed} containers"
}

cleanup_worktrees() {
    log_info "Cleaning up E2E worktrees..."

    if [[ ! -d "${WORKTREE_BASE}" ]]; then
        log_info "No worktree directory found at ${WORKTREE_BASE}"
        return 0
    fi

    local worktrees_removed=0
    local disk_freed=0

    # Find all E2E worktrees
    for worktree_path in "${WORKTREE_BASE}"/e2e-*; do
        if [[ -d "${worktree_path}" ]]; then
            local worktree_name
            worktree_name=$(basename "${worktree_path}")

            log_info "Removing worktree: ${worktree_name}"

            # Calculate size before removal
            local size
            size=$(du -sm "${worktree_path}" 2>/dev/null | cut -f1 || echo "0")
            disk_freed=$((disk_freed + size))

            # Try git worktree remove first
            if git worktree remove "${worktree_path}" --force &>/dev/null; then
                log_success "Removed worktree via git: ${worktree_name}"
                ((worktrees_removed++))
            else
                # Fall back to manual removal
                log_warning "Git worktree remove failed, trying manual removal"
                if rm -rf "${worktree_path}"; then
                    log_success "Manually removed: ${worktree_name}"
                    ((worktrees_removed++))
                else
                    log_error "Failed to remove: ${worktree_name}"
                fi
            fi
        fi
    done

    # Prune stale git worktree entries
    log_info "Pruning stale git worktree entries..."
    git worktree prune &>/dev/null || log_warning "Failed to prune worktrees"

    # Remove worktree base directory if empty
    if [[ -d "${WORKTREE_BASE}" ]]; then
        if [[ -z "$(ls -A "${WORKTREE_BASE}")" ]]; then
            rm -rf "${WORKTREE_BASE}"
            log_success "Removed empty worktree base directory"
        fi
    fi

    log_success "Removed ${worktrees_removed} worktrees, freed ${disk_freed} MB"
}

cleanup_temp_files() {
    log_info "Cleaning up temporary files..."

    local cleaned=0

    # Remove any stray diagnostics directories in project root
    if [[ -d "${PROJECT_ROOT}/diagnostics" ]]; then
        rm -rf "${PROJECT_ROOT}/diagnostics"
        ((cleaned++))
    fi

    # Remove any test cache files
    if [[ -d "${PROJECT_ROOT}/test/e2e/.cache" ]]; then
        rm -rf "${PROJECT_ROOT}/test/e2e/.cache"
        ((cleaned++))
    fi

    if [[ ${cleaned} -gt 0 ]]; then
        log_success "Cleaned ${cleaned} temporary locations"
    else
        log_info "No temporary files to clean"
    fi
}

show_summary() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                    Cleanup Summary                             ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""

    # Check for remaining resources
    local remaining_containers
    local remaining_worktrees

    remaining_containers=$(docker ps -a --filter "name=e2e-" --format "{{.Names}}" 2>/dev/null | wc -l)
    if [[ -d "${WORKTREE_BASE}" ]]; then
        remaining_worktrees=$(find "${WORKTREE_BASE}" -maxdepth 1 -name "e2e-*" -type d 2>/dev/null | wc -l)
    else
        remaining_worktrees=0
    fi

    echo "Status:"
    if [[ ${remaining_containers} -eq 0 ]]; then
        log_success "E2E containers: All cleaned ✓"
    else
        log_warning "E2E containers: ${remaining_containers} remaining"
    fi

    if [[ ${remaining_worktrees} -eq 0 ]]; then
        log_success "E2E worktrees: All cleaned ✓"
    else
        log_warning "E2E worktrees: ${remaining_worktrees} remaining"
    fi

    echo ""

    if [[ ${remaining_containers} -eq 0 && ${remaining_worktrees} -eq 0 ]]; then
        log_success "✓ All E2E resources cleaned up successfully"
    else
        log_warning "Some resources remain. You may need to clean them manually:"
        if [[ ${remaining_containers} -gt 0 ]]; then
            echo "  docker ps -a --filter 'name=e2e-'"
            echo "  docker rm -f <container-name>"
        fi
        if [[ ${remaining_worktrees} -gt 0 ]]; then
            echo "  ls -la ${WORKTREE_BASE}"
            echo "  git worktree remove <path> --force"
        fi
    fi
}

#=============================================================================
# Argument Parsing
#=============================================================================

parse_arguments() {
    if [[ $# -eq 0 ]]; then
        return 0
    fi

    case "$1" in
        --all)
            CLEANUP_WORKTREES=true
            CLEANUP_CONTAINERS=true
            ;;
        --worktrees)
            CLEANUP_WORKTREES=true
            CLEANUP_CONTAINERS=false
            ;;
        --containers)
            CLEANUP_WORKTREES=false
            CLEANUP_CONTAINERS=true
            ;;
        --help|-h)
            cat <<EOF
Usage: $0 [--all | --worktrees | --containers]

Options:
  --all         Clean up everything (default)
  --worktrees   Only clean up git worktrees
  --containers  Only clean up Docker containers
  --help        Show this help message

Examples:
  $0                  # Clean up everything
  $0 --worktrees      # Only remove worktrees
  $0 --containers     # Only stop and remove containers
EOF
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            log_info "Use --help for usage information"
            exit 1
            ;;
    esac
}

#=============================================================================
# Main Execution
#=============================================================================

main() {
    parse_arguments "$@"

    echo ""
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║           E2E Test Environment Cleanup                         ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo ""

    if [[ "${CLEANUP_CONTAINERS}" == "true" ]]; then
        cleanup_containers
    fi

    if [[ "${CLEANUP_WORKTREES}" == "true" ]]; then
        cleanup_worktrees
    fi

    cleanup_temp_files

    show_summary
}

# Run main function
main "$@"
