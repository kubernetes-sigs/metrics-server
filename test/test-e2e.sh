#!/bin/bash

set -e

: ${NODE_IMAGE:?Need to set NODE_IMAGE to test}
: ${SKAFFOLD_PROFILE:="test"}
: ${LOCAL:=false}

KIND_VERSION=0.17.0
SKAFFOLD_VERSION=1.39.1
HELM_VERSION=3.10.2
KUBECTL_VERSION=1.25.4
METRICS_SERVER_IMAGE="gcr.io/k8s-staging-metrics-server/metrics-server"

delete_cluster() {
  ${KIND} delete cluster --name=e2e &> /dev/null || true
}

setup_helm() {
  HELM=$(which helm || true)
  if [[ ${HELM} == "" || $(${HELM} |grep Version |awk -F'Version:' '{print $2}' |awk -F',' '{print $1}') != "\"v${HELM_VERSION}\"" ]] ; then
    HELM=_output/helm
  fi
  if ! [[ $(${HELM} version |grep Version |awk -F'Version:' '{print $2}' |awk -F',' '{print $1}') == "\"v${HELM_VERSION}\"" ]] ; then
      echo "helm not found or bad version, downloading binary"
      mkdir -p _output
      curl -sL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" | tar xz -C _output --strip-components 1
      chmod +x _output/helm
      HELM=_output/helm
  fi
}

setup_kind() {
  KIND=$(which kind || true)
  if [[ ${KIND} == "" || $(${KIND} --version) != "kind version ${KIND_VERSION}" ]] ; then
    KIND=_output/kind
  fi
  if ! [[ $(${KIND} --version) == "kind version ${KIND_VERSION}" ]] ; then
      echo "kind not found or bad version, downloading binary"
      mkdir -p _output
      curl -sLo _output/kind "https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-$(uname)-amd64"
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
      curl -sLo _output/skaffold "https://storage.googleapis.com/skaffold/releases/v${SKAFFOLD_VERSION}/skaffold-linux-amd64"
      chmod +x _output/skaffold
      SKAFFOLD=_output/skaffold
  fi
}

get_kubectl_version() {
  ${KUBECTL} version --client --short 2>&1 | grep 'Client Version: ' | sed 's/^.*: //'
}

setup_kubectl() {
  KUBECTL=$(which kubectl || true)
  if [[ ${KUBECTL} == "" || $(get_kubectl_version) != "v${KUBECTL_VERSION}" ]] ; then
    KUBECTL=_output/kubectl
  fi
  if ! [[ $(get_kubectl_version) == "v${KUBECTL_VERSION}" ]] ; then
      echo "kubectl not found or bad version, downloading binary"
      mkdir -p _output
      curl -sLo _output/kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
      chmod +x _output/kubectl
      KUBECTL=_output/kubectl
  fi
}

create_cluster() {
  KIND_CONFIG=""
  if [ "${SKAFFOLD_PROFILE}" = "test-ha" ] ; then
    KIND_CONFIG="$PWD/test/kind-ha-config.yaml"
  fi
  if ! (${KIND} create cluster --name=e2e --image=${NODE_IMAGE} --config=${KIND_CONFIG}) ; then
    echo "Could not create Kind cluster"
    exit 1
  fi
}

deploy_metrics_server(){
  PATH="$PWD/_output:${PATH}" ${SKAFFOLD} run -p "${SKAFFOLD_PROFILE}"
  sleep 5
}

run_tests() {
  echo "Running tests with cache disabled."
  go test test/e2e_test.go -v -count=1
}

upload_metrics_server_logs(){
  ${KUBECTL} logs -n kube-system -l "k8s-app=metrics-server" > $ARTIFACTS/metrics-server.log
}

if [ "${SKAFFOLD_PROFILE}" = "helm" ] ; then
  setup_helm
fi
setup_kind
setup_skaffold
trap delete_cluster EXIT
delete_cluster
create_cluster
deploy_metrics_server
run_tests

if [[ ${ARTIFACTS} != "" ]] ; then
  setup_kubectl
  upload_metrics_server_logs
fi

DOCKER=$(which docker || true)
if ! command -v "${DOCKER}" &> /dev/null ; then
    echo "Docker could not be found."
    exit
fi

if [ "${LOCAL}" = true ] ; then
  echo "Removing test images from local registry."
  # Force image removal to remove all tags.
  "${DOCKER}" rmi -f \
  "$("${DOCKER}" images |\
  # Skip the first line which contains the headers.
   tail -n +2 |\
  # Get the image tag.
    awk -v image="${METRICS_SERVER_IMAGE}" '($1 == image) {print $3}' |\
     head -n 1)"
fi
