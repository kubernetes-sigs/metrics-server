#!/bin/bash

set -e

: ${IMAGE:?Need to provide environment variable IMAGE}
: ${EXPECTED_VERSION:?Need to provide environment variable EXPECTED_VERSION}

CONTAINER_VERSION=$(docker run --rm ${IMAGE} --version)

if [[ "${CONTAINER_VERSION}" != "${EXPECTED_VERSION}" ]] ; then
  echo "Unexpected binary version, got ${CONTAINER_VERSION}, expected ${EXPECTED_VERSION}"
  exit 1
fi

