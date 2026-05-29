# Contributing Guidelines

Welcome to Kubernetes. We are excited about the prospect of you joining our [community](https://git.k8s.io/community)! The Kubernetes community abides by the CNCF [code of conduct](code-of-conduct.md). Here is an excerpt:

_As contributors and maintainers of this project, and in the interest of fostering an open and welcoming community, we pledge to respect all people who contribute through reporting issues, posting feature requests, updating documentation, submitting pull requests or patches, and other activities._

## Getting Started

We have full documentation on how to get started contributing here:

<!---
If your repo has certain guidelines for contribution, put them here ahead of the general k8s resources
-->

- [Contributor License Agreement](https://git.k8s.io/community/CLA.md) Kubernetes projects require that you sign a Contributor License Agreement (CLA) before we can accept your pull requests
- [Kubernetes Contributor Guide](https://git.k8s.io/community/contributors/guide) - Main contributor documentation, or you can just jump directly to the [contributing section](https://git.k8s.io/community/contributors/guide#contributing)
- [Contributor Cheat Sheet](https://git.k8s.io/community/contributors/guide/contributor-cheatsheet) - Common resources for existing developers

## Mentorship

- [Mentoring Initiatives](https://git.k8s.io/community/mentoring) - We have a diverse set of mentorship programs available that are always looking for volunteers!

## Chart Changes

When contributing chart changes please follow the same process as when contributing other content but also please **DON'T** modify _Chart.yaml_ in the PR as this would result in a chart release when merged and will mean that your PR will need modifying before it can be accepted. The chart version will be updated as part of the PR to release the chart.

## Development

Required tools:

- [Docker](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/)
- [Skaffold](https://skaffold.dev/)

## Adding dependencies

The project follows a standard Go project layout, see more about [dependency-management](https://github.com/kubernetes/community/blob/master/contributors/devel/development.md#dependency-management).

## Running static code validation

```sh
make lint
```

## Running tests

```sh
make test-unit
make test-version
make test-e2e
```

## Live reload

To start local development just run:

```sh
kind create cluster
skaffold dev -p test
```

Or, to start one using client certificates, run:

```sh
kind create cluster
mkdir -p _output/certs
openssl req -new -newkey rsa:2048 -nodes -keyout _output/certs/client.key -out _output/certs/client.csr -subj "/CN=metrics-server-client"
kubectl apply -f - <<EOF
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: metrics-server-client
spec:
  request: $(cat _output/certs/client.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  expirationSeconds: 86400  # 24*60*60
  usages:
  - client auth
EOF
kubectl certificate approve metrics-server-client
kubectl get csr metrics-server-client -o jsonpath='{.status.certificate}' | base64 -d > manifests/components/test-client-certs/client.crt
cp _output/certs/client.key manifests/components/test-client-certs/client.key
skaffold dev -p test-client-certs
```
