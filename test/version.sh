#!/bin/bash

set -e

: ${IMAGE:?Need to set metrics-server IMAGE variable to test}
: ${EXPECTED_VERSION:?Need to set EXPECTED_VERSION variable to test}

CONTAINER_VERSION=$(docker run --rm ${IMAGE} --version)

if [[ "${CONTAINER_VERSION}" != "${EXPECTED_VERSION}" ]] ; then
  echo "Unextected binary version, got ${CONTAINER_VERSION}, expected ${EXPECTED_VERSION}"
  exit 1
fi

