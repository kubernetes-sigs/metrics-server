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

# Development

Required tools:
* [Docker](https://www.docker.com/)
* [Kind](https://kind.sigs.k8s.io/)
* [Skaffold](https://skaffold.dev/)

## Adding dependencies

The project follows a standard Go project layout, see more about [dependency-management](https://github.com/kubernetes/community/blob/master/contributors/devel/development.md#dependency-management).

## Running static code validation

```
make lint
```

## Running tests

```
make test-unit
make test-version
make test-e2e
```

## Live reload

To start local development just run:
```
kind create cluster
skaffold dev
```

To execute e2e tests run:
```
go test test/e2e_test.go -v -count=1
```

## Perf-test

The performance test of metrics-server depends on the `clusterLoader` tool in `kubernetes/perf-tests` repository.

`clusterLoader` supports prometheus to scrape metrics from metrics-server to achieve metrics-server latency measurement.

### Design principle

* metrics-server implements two APIs for performance testing, MetricsServerPrometheus and InClusterAPIServerRequestLatency.
  The implementation of MetricsServerPrometheus is basically the same as that of kube-state-metrics, done in the prometheus framework of `clusterloader`.
  The implementation of InClusterAPIServerRequestLatency, done in the probes about measurement framework of `clusterloader`.

* According to the performance testing framework of `clusterLoader`, metrics-server implements two actions here

    * `start` Complete the preparations for the start of the test

      MetricsServerPrometheus: Record start time
      InClusterAPIServerRequestLatency: Record start time and access the PodMetrics resource periodically.

    * `gather`
      Collect metrics-server metrics data through prometheus to achieve metrics server latency measurement

### Use cases
* Deploy kubernetes cluster


* Clone perf-tests repository

  Prepare the go environment
  Start with cloning perf-tests repository:

```
  git clone git@github.com:kubernetes/perf-tests.git
  cd perf-tests/clusterloader2
```

Note: the code needs to be cloned to the gopath directory.

* Deploy metrics-server


* Run metrics server latency measurement

```
./run-e2e.sh cluster-loader2 --provider=xxx --enable-prometheus-server=true  --prometheus-scrape-metrics-server=true --prometheus-apiserver-scrape-port=6443  --testconfig=metrics-server-scrape-config.yaml --prometheus-manifest-path=pkg/prometheus/manifests

```
The format of metrics-server-scrape-config.yaml is as follows
```yaml
steps:
- name: Start Measurements
  measurements:
  - Identifier: MetricsServerPrometheus
    Method: MetricsServerPrometheus
    Params:
      action: start
- name: Start Probe
  measurements:
  - Identifier: MetricsServerProber
    Method: InClusterAPIServerRequestLatency
    Params:
      action: start
      checkProbesReadyTimeout: 60s
      replicasPerProbe: 1
- name: Sleep
  measurements:
  - Identifier: sleep
    Method: Sleep
    Params:
      duration: 60s
- name: Measurement Probe
  measurements:
  - Identifier: MetricsServerProber  
    Method: InClusterAPIServerRequestLatency
    Params:
      action: gather
- name: Measure access metricsServer latency
  measurements:
  - Identifier: MetricsServerPrometheus
    Method: MetricsServerPrometheus
    Params:
      action: gather

```

Note: `--provider` supports gce and kind

In kind mode, k8s components are required to listen to all 0 addresses

config example of kind
```yaml
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    controllerManager:
      extraArgs:
        bind-address: "0.0.0.0" 
    scheduler:
      extraArgs:
        bind-address: "0.0.0.0"

```


* Result

  A sample result is as follows

```json
I0514 15:18:26.866500 2094855 simple_test_executor.go:87] InClusterAPIServerRequestLatency: {
  "version": "v1",
  "dataItems": [
    {
      "data": {
        "Perc50": xxx,
        "Perc90": xxx,
        "Perc99": xxx
      },
      "unit": "ms",
      "labels": {
        "Metric": "InClusterAPIServerRequestLatency"
      }
    }
  ]
}
I0514 15:18:26.866535 2094855 simple_test_executor.go:87] MetricsServerPrometheus: {
  "Perc50": xxx,
  "Perc90": xxx,
  "Perc99": xxx
}

```