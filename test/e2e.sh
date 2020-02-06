#!/bin/bash

set -e

: ${IMAGE:?Need to set metrics-server IMAGE variable to test}
: ${KUBERNETES_VERSION:?Need to set KUBERNETES_VERSION to test}

cleanup() {
  kind delete cluster --name=e2e-${KUBERNETES_VERSION} &> /dev/null || true
}

setup_kind() {
  GO111MODULE="on" go get sigs.k8s.io/kind@v0.6.0
  export PATH=$(go env GOPATH)/bin:$PATH

  cleanup

  if ! (kind create cluster --name=e2e-${KUBERNETES_VERSION} --image=kindest/node:${KUBERNETES_VERSION}) ; then
    echo "Could not create KinD cluster"
    exit 1
  fi
  kind load docker-image ${IMAGE} --name e2e-${KUBERNETES_VERSION}
}

deploy(){
  kubectl apply -f deploy/kubernetes/
  # Apply patch to use provided image
  kubectl -n kube-system patch deployment metrics-server --patch "{\"spec\": {\"template\": {\"spec\": {\"containers\": [{\"name\": \"metrics-server\", \"image\": \"${IMAGE}\", \"imagePullPolicy\": \"Never\"}]}}}}"
  # Configure metrics-server preffered address to InternalIP for it to work with KinD
  kubectl -n kube-system patch deployment metrics-server --patch '{"spec": {"template": {"spec": {"containers": [{"name": "metrics-server", "args": ["--cert-dir=/tmp", "--secure-port=4443", "--kubelet-preferred-address-types=InternalIP", "--kubelet-insecure-tls"]}]}}}}'
}

wait_for_metrics() {
  # Wait for metrics pod ready
  while [[ $(kubectl get pods -n kube-system -l k8s-app=metrics-server -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]; do
    echo "waiting for pod ready" && sleep 5;
  done

  # By default metrics server scrapes every 60s
  sleep 60
}

run_tests() {
  GO111MODULE=on go test test/e2e_test.go -v
}

trap cleanup EXIT
setup_kind
deploy
wait_for_metrics
run_tests
