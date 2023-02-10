#!/bin/bash

set -e

METRICS_SERVER_IMAGE="gcr.io/k8s-staging-metrics-server/metrics-server"
DOCKER=$(which docker || true)
if ! command -v "${DOCKER}" &> /dev/null ; then
    echo "Docker could not be found, exiting."
    exit
fi
