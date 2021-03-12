## FAQ

## Table of Contents

<!-- toc -->
- [What metrics are exposed by the metrics server?](#what-metrics-are-exposed-by-the-metrics-server)
- [How CPU usage is calculated?](#how-cpu-usage-is-calculated)
- [How memory usage is calculated?](#how-memory-usage-is-calculated)
- [How does the metrics server calculate metrics?](#how-does-the-metrics-server-calculate-metrics)
- [How often is metrics server released?](#how-often-is-metrics-server-released)
- [Can I run two instances of metrics-server?](#can-i-run-two-instances-of-metrics-server)
- [How to run metrics-server securely?](#how-to-run-metrics-server-securely)
- [How to run metric-server on different architecture?](#how-to-run-metric-server-on-different-architecture)
- [What Kubernetes versions are supported?](#what-kubernetes-versions-are-supported)
- [How is resource utilization calculated?](#how-is-resource-utilization-calculated)
- [How to autoscale Metrics Server?](#how-to-autoscale-metrics-server)
- [Can I get other metrics beside CPU/Memory using Metrics Server?](#can-i-get-other-metrics-beside-cpumemory-using-metrics-server)
- [How large can clusters be?](#how-large-can-clusters-be)
- [How often metrics are scraped?](#how-often-metrics-are-scraped)
<!-- /toc -->

#### What metrics are exposed by the metrics server?

Metrics server collects resource usage metrics needed for autoscaling: CPU & Memory.
Metric values use standard kubernetes units (`m`, `Ki`), same as those used to
define pod requests and limits (Read more [Meaning of CPU], [Meaning of memory])
Metrics server itself is not responsible for calculating metric values, this is done by Kubelet.

#### How CPU usage is calculated?

CPU is reported as the average usage, in CPU cores, over a period of time.
This value is derived by taking a rate over a cumulative CPU counter provided by the kernel (in both Linux and Windows kernels).
The kubelet chooses the window for the rate calculation.

#### How memory usage is calculated?

Memory is reported as the working set at the instant the metric was collected.
In an ideal world, the "working set" is the amount of memory in-use that cannot be freed under memory pressure.
However, calculation of the working set varies by host OS, and generally makes heavy use of heuristics to produce an estimate.
It includes all anonymous (non-file-backed) memory since Kubernetes does not support swap.
The metric typically also includes some cached (file-backed) memory, because the host OS cannot always reclaim such pages.

#### How does the metrics server calculate metrics?

Metrics Server itself doesn't calculate any metrics, it aggregates values exposed by Kubelet and exposes them in API
to be used for autoscaling. For any problem with metric values please contact SIG-Node.

#### How often is metrics server released?

There is no hard release schedule. A release is done after an important feature is implemented or upon request.

#### Can I run two instances of metrics-server?

Yes, but it will not provide any benefits. Both instances will scrape all nodes to collect metrics, but only one instance will be actively serving metrics API.

#### How to run metrics-server securely?

Suggested configuration:
* Cluster with [RBAC] enabled
* Kubelet [read-only port] port disabled
* Validate kubelet certificate by mounting CA file and providing `--kubelet-certificate-authority` flag to metrics server
* Avoid passing insecure flags to metrics server (`--deprecated-kubelet-completely-insecure`, `--kubelet-insecure-tls`)
* Consider using your own certificates (`--tls-cert-file`, `--tls-private-key-file`)

#### How to run metric-server on different architecture?

Starting from `v0.3.7` docker image `k8s.gcr.io/metrics-server/metrics-server` should support multiple architectures via Manifests List.
List of supported architectures: `amd64`, `arm`, `arm64`, `ppc64le`, `s390x`.

#### What Kubernetes versions are supported?

Metrics server is tested against the last 3 Kubernetes versions.

#### How is resource utilization calculated?

Metrics server doesn't provide resource utilization metrics (e.g. percent of CPU used).
Utilization presented by `kubectl top` and HPA is calculated client side based on pod resource requests or node capacity.

#### How to autoscale Metrics Server?

Metrics server scales linearly vertically according to the number of nodes and pods in a cluster. This can be automated using [addon-resizer].

#### Can I get other metrics beside CPU/Memory using Metrics Server?

No, metrics server was designed to provide metrics for [resource metrics pipeline] used for autoscaling.

#### How large can clusters be?

Metrics Server was tested to run within clusters up to 5000 nodes with an average pod density of 30 pods per node.

#### How often metrics are scraped?

Default 60 seconds, can be changed using `metric-resolution` flag. We are not recommending setting values below 15s, as this is the resolution of metrics calculated by Kubelet.

[Meaning of CPU]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
[Meaning of memory]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[read-only port]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/#options
[addon-resizer]: https://github.com/kubernetes/autoscaler/tree/master/addon-resizer
[resource metrics pipeline]: https://kubernetes.io/docs/tasks/debug-application-cluster/resource-metrics-pipeline/
