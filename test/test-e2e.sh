#!/bin/bash

set -e

: ${NODE_IMAGE:?Need to set NODE_IMAGE to test}
: ${HIGH_AVAILABILITY:-false}

KIND_VERSION=0.11.1
SKAFFOLD_VERSION=1.32.0

delete_cluster() {
  ${KIND} delete cluster --name=e2e &> /dev/null || true
}

setup_kind() {
  KIND=$(which kind || true)
  if [[ ${KIND} == "" || $(${KIND} --version) != "kind version ${KIND_VERSION}" ]] ; then
    KIND=_output/kind
  fi
  if ! [[ $(${KIND} --version) == "kind version ${KIND_VERSION}" ]] ; then
      echo "kind not found or bad version, downloading binary"
      mkdir -p _output
      curl -Lo _output/kind "https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-$(uname)-amd64"
      chmod +x _output/kind
      KIND=_output/kind
  fi
}

setup_skaffold() {
  SKAFFOLD=$(which skaffold || true)
  if [[ ${SKAFFOLD} == "" || $(${SKAFFOLD} version) != "v${SKAFFOLD_VERSION}" ]] ; then
    SKAFFOLD=_output/skaffold
  fi
  if ! [[ $(${SKAFFOLD} version) == "v${SKAFFOLD_VERSION}" ]] ; then
      echo "skaffold not found or bad version, downloading binary"
      mkdir -p _output
      curl -Lo _output/skaffold "https://storage.googleapis.com/skaffold/releases/v${SKAFFOLD_VERSION}/skaffold-linux-amd64"
      chmod +x _output/skaffold
      SKAFFOLD=_output/skaffold
  fi
}

create_cluster() {
  KIND_CONFIG=""
  if [ "${HIGH_AVAILABILITY}" = true ] ; then
    KIND_CONFIG="$PWD/test/kind-ha-config.yaml"
  fi
  if ! (${KIND} create cluster --name=e2e --image=${NODE_IMAGE} --config=${KIND_CONFIG}) ; then
    echo "Could not create KinD cluster"
    exit 1
  fi
}

deploy_metrics_server(){
  SKAFFOLD_PROFILE=""
  if [ "${HIGH_AVAILABILITY}" = true ] ; then
    SKAFFOLD_PROFILE="test-ha"
  fi
  PATH="$PWD/_output:${PATH}" ${SKAFFOLD} run -p "${SKAFFOLD_PROFILE}"
  sleep 5
}

run_tests() {
  go test test/e2e_test.go -v -count=1
}

setup_kind
setup_skaffold
trap delete_cluster EXIT
delete_cluster
create_cluster
deploy_metrics_server
run_tests
