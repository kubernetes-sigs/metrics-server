#!/bin/bash

set -e

source hack/scripts/export-variables.sh

echo "Removing test images from local registry."
# Force image removal to remove all tags.
"${DOCKER}" rmi -f \
  "$("${DOCKER}" images |\
    # Skip the first line which contains the headers.
    tail -n +2 |\
    # Get the image tag.
      awk -v image="${METRICS_SERVER_IMAGE}" '($1 == image) {print $3}' | head -n 1)"
