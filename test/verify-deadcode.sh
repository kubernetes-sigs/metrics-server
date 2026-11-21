#!/bin/bash

# This script verifies that the golang linker is eliminating dead code from
# the metrics-server binary, e.g. that grpc's optional request tracing
# (golang.org/x/net/trace, text/template, ...) is not linked in.
#
# Usage: `test/verify-deadcode.sh`.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}"

BINARY_DIR=$(mktemp -d)
trap 'rm -rf "${BINARY_DIR}"' EXIT

output=$(OUTPUT_DIR="${BINARY_DIR}" BINARY_NAME=metrics-server GOLDFLAGS=-dumpdep make build 2>&1 | grep "\->" | go tool github.com/aarzilli/whydeadcode 2>&1)

if [[ -n "$output" ]]; then
  echo "golang linker is not eliminating dead code, please check the trace output below:"
  echo "(NOTE: there may be false positives, but the first trace should be a real issue)"
  echo "$output"
  exit 1
fi

echo "verify-deadcode passed"
