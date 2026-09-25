#!/usr/bin/env bash
#
# Generates or checks the capability baseline for go-toml.
#
# Usage:
#   ./caps.sh generate   # regenerate capability_baseline.txt
#   ./caps.sh check      # check that capabilities haven't grown
#
# Requires: go, capslock (go install github.com/google/capslock/cmd/capslock@latest)

set -euo pipefail

BASELINE="capability_baseline.txt"
CAPSLOCK="${CAPSLOCK:-capslock}"

# Capabilities that must never appear in any package.
FORBIDDEN_CAPS=(
    CAPABILITY_NETWORK
    CAPABILITY_CGO
    CAPABILITY_EXEC
)

capslock_to_baseline() {
    "$CAPSLOCK" -packages=. -output=package -granularity=package \
        | jq -r 'to_entries | sort_by(.key) | .[] | .key + ": " + (.value | sort | join(", "))'
}

generate() {
    capslock_to_baseline > "$BASELINE"
    echo "Wrote $BASELINE"
}

check() {
    if [ ! -f "$BASELINE" ]; then
        echo "ERROR: $BASELINE not found. Run '$0 generate' first."
        exit 1
    fi

    current=$(mktemp)
    trap 'rm -f "$current"' EXIT

    capslock_to_baseline > "$current"

    failed=0

    # Check for forbidden capabilities in current output.
    for cap in "${FORBIDDEN_CAPS[@]}"; do
        if grep -q "$cap" "$current"; then
            echo "FORBIDDEN capability found: $cap"
            grep "$cap" "$current"
            failed=1
        fi
    done

    # Extract all unique capability names from baseline and current.
    baseline_caps=$(grep -oE 'CAPABILITY_[A-Z_]+' "$BASELINE" | sort -u)
    current_caps=$(grep -oE 'CAPABILITY_[A-Z_]+' "$current" | sort -u)

    # Check for new capability names not in the baseline.
    new_caps=$(comm -13 <(echo "$baseline_caps") <(echo "$current_caps"))
    if [ -n "$new_caps" ]; then
        echo "NEW capabilities detected (not in baseline):"
        echo "$new_caps"
        failed=1
    fi

    # Check for new per-package capabilities (a package gained a capability it didn't have before).
    while IFS=': ' read -r pkg caps; do
        baseline_pkg_caps=$(grep "^${pkg}:" "$BASELINE" 2>/dev/null | sed 's/^[^:]*: //' || true)
        if [ -z "$baseline_pkg_caps" ]; then
            echo "NEW package with capabilities: $pkg: $caps"
            failed=1
            continue
        fi
        # Check each capability in current for this package
        for cap in $(echo "$caps" | tr ', ' '\n' | grep -v '^$'); do
            if ! echo "$baseline_pkg_caps" | grep -q "$cap"; then
                echo "NEW capability for $pkg: $cap"
                failed=1
            fi
        done
    done < "$current"

    if [ "$failed" -eq 1 ]; then
        echo ""
        echo "FAILED: capabilities have grown."
        echo "If this is intentional, run '$0 generate' and commit the updated $BASELINE."
        exit 1
    fi

    echo "OK: no new capabilities detected."
}

case "${1:-}" in
    generate) generate ;;
    check)    check ;;
    *)
        echo "Usage: $0 {generate|check}"
        exit 1
        ;;
esac
