# Kubernetes Metrics Server

This project is maintained by [sig-instrumentation](https://github.com/kubernetes/community/tree/master/sig-instrumentation)

## User guide

You can find the user guide in
[the official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug-application-cluster/resource-metrics-pipeline/).

## Design

The detailed design of the project can be found in the following docs:

- [Metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md)
- [Metrics Server](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/metrics-server.md)

For the broader view of monitoring in Kubernetes take a look into
[Monitoring architecture](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/monitoring_architecture.md)

## Requirements

Metrics server has particular requirements on cluster and network configuration that is not default for all cluster distributions. Please ensure that your cluster distribution supports those requirements before using metrics server:
* Metrics server needs to be reachable from kube-apiserver ([Configuring master to cluster communication](https://kubernetes.io/docs/concepts/architecture/master-node-communication/#master-to-cluster))
* Kube-apiserver should be correctly configured to enable aggregation layer ([How to configure aggregation layer](https://kubernetes.io/docs/tasks/access-kubernetes-api/configure-aggregation-layer/))
* Nodes need to have kubelet authorization configured and match metrics-server configuration ([How to configure kubelet authorization](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-authentication-authorization/))
* Pod/Node metrics need to be exposed by Kubelet by Summary API

## Deployment

Compatibility matrix:

Metrics Server | Metrics API group/version | Supported Kubernetes version
---------------|---------------------------|-----------------------------
0.3.x          | `metrics.k8s.io/v1beta1`  | 1.8+
0.2.x          | `metrics.k8s.io/v1beta1`  | 1.8+


In order to deploy metrics-server in your cluster run the following command from
the top-level directory of this repository:

```console
# Kubernetes > 1.8
$ kubectl create -f deploy/kubernetes/
```

You can also use this helm chart to deploy the metric-server in your cluster (This isn't supported by the metrics-server maintainers): https://github.com/helm/charts/tree/master/stable/metrics-server

## FAQ

#### What metrics are exposed by metrics server?

Metrics server collects resource usage metrics needed for autoscaling: CPU & Memory.
Metric values use standard kubernetes units (`m`, `Ki`), same as those used to
define pod requests and limits (Read more [Meaning of CPU](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu), [Meaning of memory](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory))
Metrics server itself is not responsible for calculating metric values, this is done by Kubelet.

#### When metrics server is released?

There is no hard release schedule. Release is done after important feature is implemented or upon request.

#### Can I run two instances of metrics-server?

Yes, but it will not provide any benefits. Both instances will scrape all nodes to collect metrics, but only one instance will be actively serving metrics API.

#### How to run metrics-server securely?

Suggested configuration:
* Cluster with [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) enabled
* Kubelet [read-only](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/#options) port disabled
* Validate kubelet certificate by mounting CA file and providing `--kubelet-certificate-authority` flag to metrics server
* Avoid passing insecure flags to metrics server (`--deprecated-kubelet-completely-insecure`, `--kubelet-insecure-tls`)
* Consider using your own certificates (`--tls-cert-file`, `--tls-private-key-file`)

#### How to run metric-server on different architecture?

There are officially built images for `amd64`, `arm`, `arm64`, `ppc64le`, `s390x`. Please update manifests to use specific image e.g. `k8s.gcr.io/metrics-server-s390x:v0.3.6`

#### What Kubernetes versions are supported?

Metrics server is tested against last 3 Kubernetes versions.

#### How resource utilization is calculated?

Metrics server doesn't provide resource utilization metric (e.g. percent of CPU used).
Kubectl top and HPA calculate those values by themselves based on pod resource requests or node capacity.

#### How to autoscale Metrics Server?

Metrics server scales linearly vertically to number of nodes and pods in cluster. This can be automated using [addon-resizer](https://github.com/kubernetes/autoscaler/tree/master/addon-resizer)

#### Why metrics values differ from one collected by Prometheus?

Values differ as they are used for different purpose.
Metrics server CPU metric is used for horizontal autoscaling, that's why it represents latest values (last 15s), Prometheus cares about average usage.
Metrics server memory metric is used for vertical autoscaling, that's why it represents memory used by Kubelet for OOMing (Working Set), Prometheus cares about usage.

#### Can I get other metrics beside CPU/Memory using Metrics Server?

No, metrics server was designed to provide metrics used for autoscaling.

#### What requests and limits I should set for metrics server?

Metrics server scales linearly if number of nodes and pods in cluster. For pod density of 30 pods per node:

* CPU: 40mCore base + 0.5 mCore per node
* Memory: 40MiB base + 4 MiB per node

For higher pod density you should be able to scale resources proportionally.
We are not recommending setting CPU limits as metrics server needs more compute to generate certificates at bootstrap.

#### How big clusters are supported?

Metrics Server was tested to run within clusters up to 5000 nodes with average pod density of 30 pods per node.

#### How often metrics are scraped?

Default 60 seconds, can be changed using `metrics-resolution` flag. We are not recommending setting values below 15s, as this is the resolution of metrics calculated within Kubelet.

## Flags

Metrics Server supports all the standard Kubernetes API server flags, as
well as the standard Kubernetes `glog` logging flags.  The most
commonly-used ones are:

- `--logtostderr`: log to standard error instead of files in the
  container.  You generally want this on.

- `--v=<X>`: set log verbosity.  It's generally a good idea to run a log
  level 1 or 2 unless you're encountering errors.  At log level 10, large
  amounts of diagnostic information will be reported, include API request
  and response bodies, and raw metric results from Kubelet.

- `--secure-port=<port>`: set the secure port.  If you're not running as
  root, you'll want to set this to something other than the default (port
  443).

- `--tls-cert-file`, `--tls-private-key-file`: the serving certificate and
  key files.  If not specified, self-signed certificates will be
  generated, but it's recommended that you use non-self-signed
  certificates in production.

- `--kubelet-certificate-authority`: the path of the CA certificate to use 
  for validate the Kubelet's serving certificates.

Additionally, Metrics Server defines a number of flags for configuring its
behavior:

- `--metric-resolution=<duration>`: the interval at which metrics will be
  scraped from Kubelets (defaults to 60s).

- `--kubelet-insecure-tls`: skip verifying Kubelet CA certificates.  Not
  recommended for production usage, but can be useful in test clusters
  with self-signed Kubelet serving certificates.

- `--kubelet-port`: the port to use to connect to the Kubelet (defaults to
  the default secure Kubelet port, 10250).

- `--kubelet-preferred-address-types`: the order in which to consider
  different Kubelet node address types when connecting to Kubelet.
  Functions similarly to the flag of the same name on the API server.

# Development

## Runing e2e tests

Pre-requesites:
* Docker
* kubectl

Run:
```
make test-e2e
```