#!/usr/bin/env bash
# Compares the Go VM with qrvmone on the conformance corpus in
# core/vm/conformance. Builds the qrvmone-conformance runner from a qrvmone
# checkout and runs TestGoVMMatchesQrvmone against it.
#
# Usage: QRVMONE_DIR=/path/to/qrvmone hack/vm-conformance.sh
# Optional: QRVMONE_BUILD_DIR (default $QRVMONE_DIR/build, which qrvmone ignores).
set -euo pipefail

QRVMONE_DIR=${QRVMONE_DIR:?set QRVMONE_DIR to a qrvmone checkout with submodules}
BUILD_DIR=${QRVMONE_BUILD_DIR:-$QRVMONE_DIR/build}
REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)

cmake -S "$QRVMONE_DIR" -B "$BUILD_DIR" -DCMAKE_BUILD_TYPE=Release -DQRVMONE_TESTING=ON
cmake --build "$BUILD_DIR" --target qrvmone-conformance

cd "$REPO_ROOT"
QRVMONE_CONFORMANCE_RUNNER="$BUILD_DIR/bin/qrvmone-conformance" \
	go test -count=1 -run TestGoVMMatchesQrvmone -v ./core/vm/conformance
