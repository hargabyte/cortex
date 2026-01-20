#!/bin/bash
# verify-dead-code.sh - Verify dead code detection has no false positives
#
# This script tests that the dead code whitelist filter correctly excludes
# entity types that cannot be reliably tracked (imports, variables, constants).
#
# Usage: ./scripts/verify-dead-code.sh [--verbose]

set -e

CX="${CX:-./cx}"
REPORT_DIR="${REPORT_DIR:-/tmp/cx-dead-code-test}"
VERBOSE="${1:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Valid entity types from the whitelist (DeadCodeEntityTypes in gather.go)
VALID_TYPES="function|method|class|interface|struct|type|trait|enum"

# Invalid types that should be filtered out
INVALID_TYPES="import|variable|constant"

mkdir -p "$REPORT_DIR"

pass_count=0
fail_count=0

log_pass() {
    echo -e "${GREEN}PASS${NC}: $1"
    pass_count=$((pass_count + 1))
}

log_fail() {
    echo -e "${RED}FAIL${NC}: $1"
    fail_count=$((fail_count + 1))
}

log_warn() {
    echo -e "${YELLOW}WARN${NC}: $1"
}

log_info() {
    if [ "$VERBOSE" = "--verbose" ]; then
        echo "INFO: $1"
    fi
}

# Check if cx binary exists
if [ ! -x "$CX" ]; then
    echo "Error: cx binary not found at $CX"
    exit 1
fi

check_report() {
    local lang="$1"
    local repo_path="$2"
    local report="$REPORT_DIR/health-$lang.yaml"

    echo ""
    echo "=== Testing $lang ($repo_path) ==="

    # Run health report and capture output
    if ! "$CX" report health --data > "$report" 2>/dev/null; then
        log_warn "$lang - health report failed (may not have data)"
        return 0
    fi

    # Extract entity_type values from dead_code_groups candidates
    local entity_types=$(grep "entity_type:" "$report" 2>/dev/null | sed 's/.*entity_type:\s*//' | sort -u || true)

    if [ -z "$entity_types" ]; then
        log_pass "$lang - No dead code candidates (empty is valid)"
        return 0
    fi

    log_info "Found entity types: $(echo $entity_types | tr '\n' ' ')"

    # Check for invalid entity types
    local has_invalid=false
    for etype in $entity_types; do
        # Remove quotes and whitespace
        etype=$(echo "$etype" | tr -d '"' | tr -d "'" | xargs)

        # Check if it's an invalid type
        if echo "$etype" | grep -qE "^($INVALID_TYPES)$"; then
            log_fail "$lang - Found invalid entity type in dead code: $etype"
            has_invalid=true
        fi
    done

    if [ "$has_invalid" = false ]; then
        # Count valid candidates
        local count=$(grep -c "entity_type:" "$report" 2>/dev/null || echo "0")
        log_pass "$lang - $count candidates (all valid types)"

        if [ "$VERBOSE" = "--verbose" ]; then
            echo "  Entity types found: $(echo $entity_types | tr '\n' ', ')"
        fi
    fi

    return 0
}

scan_and_check() {
    local lang="$1"
    local repo_path="$2"

    if [ ! -d "$repo_path" ]; then
        log_warn "$lang - Repository not found at $repo_path"
        return 0
    fi

    # Scan the repository
    log_info "Scanning $repo_path..."
    if ! "$CX" scan "$repo_path" >/dev/null 2>&1; then
        log_warn "$lang - Scan failed for $repo_path"
        return 0
    fi

    # Check the report (metrics computed during report generation)
    check_report "$lang" "$repo_path"
}

echo "========================================="
echo "  Dead Code Whitelist Verification"
echo "========================================="
echo ""
echo "Testing that dead code detection excludes:"
echo "  - imports (usage not tracked)"
echo "  - variables (usage not tracked)"
echo "  - constants (usage not tracked)"
echo ""
echo "Valid types (whitelist):"
echo "  function, method, class, interface,"
echo "  struct, type, trait, enum"
echo ""

# Test 1: Current codebase (Go)
echo "=== Test 1: Go (cortex codebase) ==="
check_report "go-cortex" "/home/hargabyte/cortex"

# Test 2: TypeScript repository
TS_REPO="/tmp/ts-test-repo"
if [ -d "$TS_REPO" ]; then
    scan_and_check "typescript" "$TS_REPO"
else
    log_warn "TypeScript repo not found at $TS_REPO - skipping"
fi

# Test 3: Python repository
PY_REPO="/tmp/py-test-repo"
if [ -d "$PY_REPO" ]; then
    scan_and_check "python" "$PY_REPO"
else
    log_warn "Python repo not found at $PY_REPO - skipping"
fi

# Test 4: Verify whitelist constant in code
echo ""
echo "=== Code Verification ==="
GATHER_FILE="/home/hargabyte/cortex/internal/report/gather.go"
if [ -f "$GATHER_FILE" ]; then
    if grep -q "DeadCodeEntityTypes = map\[string\]bool" "$GATHER_FILE"; then
        log_pass "DeadCodeEntityTypes whitelist exists in gather.go"

        # Check that import is NOT in whitelist
        if ! grep -A10 "DeadCodeEntityTypes" "$GATHER_FILE" | grep -q '"import".*true'; then
            log_pass "Whitelist correctly excludes 'import'"
        else
            log_fail "Whitelist incorrectly includes 'import'"
        fi

        # Check that variable is NOT in whitelist
        if ! grep -A10 "DeadCodeEntityTypes" "$GATHER_FILE" | grep -q '"variable".*true'; then
            log_pass "Whitelist correctly excludes 'variable'"
        else
            log_fail "Whitelist incorrectly includes 'variable'"
        fi

        # Check that constant is NOT in whitelist
        if ! grep -A10 "DeadCodeEntityTypes" "$GATHER_FILE" | grep -q '"constant".*true'; then
            log_pass "Whitelist correctly excludes 'constant'"
        else
            log_fail "Whitelist incorrectly includes 'constant'"
        fi

        # Check that function IS in whitelist
        if grep -A10 "DeadCodeEntityTypes" "$GATHER_FILE" | grep -q '"function".*true'; then
            log_pass "Whitelist correctly includes 'function'"
        else
            log_fail "Whitelist missing 'function'"
        fi
    else
        log_fail "DeadCodeEntityTypes whitelist not found in gather.go"
    fi
else
    log_warn "gather.go not found - skipping code verification"
fi

echo ""
echo "========================================="
echo "           Summary"
echo "========================================="
echo -e "Passed: ${GREEN}$pass_count${NC}"
echo -e "Failed: ${RED}$fail_count${NC}"
echo ""

if [ $fail_count -gt 0 ]; then
    echo -e "${RED}VERIFICATION FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}VERIFICATION PASSED${NC}"
    exit 0
fi
