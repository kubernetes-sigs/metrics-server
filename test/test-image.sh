#!/bin/bash

set -e

: ${IMAGE:?Need to provide environment variable IMAGE}
: ${EXPECTED_ARCH:?Need to provide variable EXPECTED_ARCH}
: ${EXPECTED_VERSION:?Need to provide environment variable EXPECTED_VERSION}

IMAGE_ARCH=$(docker inspect ${IMAGE} | jq -r ".[].Architecture")

if [[ "${IMAGE_ARCH}" != "${EXPECTED_ARCH}" ]] ; then
  echo "Unexpected architecture, got ${IMAGE_ARCH}, expected ${EXPECTED_ARCH}"
  exit 1
fi

CONTAINER_VERSION=$(docker run --rm ${IMAGE} --version)

if [[ "${CONTAINER_VERSION}" != "${EXPECTED_VERSION}" ]] ; then
  echo "Unexpected binary version, got ${CONTAINER_VERSION}, expected ${EXPECTED_VERSION}"
  exit 1
fi

CLI_HELP="$(docker run --rm ${IMAGE} --help | sed 's/[ \t]*$//')"
EXPECTED_CLI_HELP="$(cat ./docs/command-line-flags.txt)"

DIFF=$(diff -u <(echo "${EXPECTED_CLI_HELP}") <(echo "${CLI_HELP}") | tail -n +3 || true)
if [ "$DIFF" ]; then
  echo "Unexpected cli help, diff:"
  echo "$DIFF"
  exit 1
fi
