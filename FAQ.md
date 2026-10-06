# FAQ

## Table of Contents

<!-- toc -->
- [What metrics are exposed by the metrics server?](#what-metrics-are-exposed-by-the-metrics-server)
- [How CPU usage is calculated?](#how-cpu-usage-is-calculated)
- [How memory usage is calculated?](#how-memory-usage-is-calculated)
- [How does the metrics server calculate metrics?](#how-does-the-metrics-server-calculate-metrics)
- [How often is metrics server released?](#how-often-is-metrics-server-released)
- [Can I run more than one instance of metrics-server?](#can-i-run-more-than-one-instance-of-metrics-server)
- [How to run metrics-server securely?](#how-to-run-metrics-server-securely)
- [How to secure the connection between kube-apiserver and Metrics Server?](#how-to-secure-the-connection-between-kube-apiserver-and-metrics-server)
- [How to run metric-server on different architecture?](#how-to-run-metric-server-on-different-architecture)
- [What Kubernetes versions are supported?](#what-kubernetes-versions-are-supported)
- [How is resource utilization calculated?](#how-is-resource-utilization-calculated)
- [How to autoscale Metrics Server?](#how-to-autoscale-metrics-server)
- [Can I get other metrics beside CPU/Memory using Metrics Server?](#can-i-get-other-metrics-beside-cpumemory-using-metrics-server)
- [How large can clusters be?](#how-large-can-clusters-be)
- [How often metrics are scraped?](#how-often-metrics-are-scraped)
<!-- /toc -->

### What metrics are exposed by the metrics server?

Metrics server collects resource usage metrics needed for autoscaling: CPU & Memory.
Metrics values use [Metric System prefixes] (`n` = 10<sup>-9</sup> and `Ki` = 2<sup>10</sup>),
the same as those used to define pod requests and limits.
Metrics server itself is not responsible for calculating metric values, this is done by Kubelet.

[Metric System prefixes]: https://en.wikipedia.org/wiki/Metric_prefix

### How CPU usage is calculated?

CPU is reported as the average core usage measured in cpu units.
One cpu, in Kubernetes, is equivalent to 1 vCPU/Core for cloud providers and 1 hyperthread on bare-metal Intel processors.

This value is derived by taking a rate over a cumulative CPU counter provided by the kernel (in both Linux and Windows kernels).
Time window used to calculate CPU is exposed under `window` field in Metrics API.

Read more about [Meaning of CPU].

[Meaning of CPU]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu

### How memory usage is calculated?

Memory is reported as the working set at the instant the metric was collected, measured in bytes.

In an ideal world, the "working set" is the amount of memory in-use that cannot be freed under memory pressure.
However, calculation of the working set varies by host OS, and generally makes heavy use of heuristics to produce an estimate.
It includes all anonymous (non-file-backed) memory since Kubernetes does not enable swap for containers by default.
The metric typically also includes some cached (file-backed) memory, because the host OS cannot always reclaim such pages.

Read more about [Meaning of memory].

[Meaning of memory]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory

### How does the metrics server calculate metrics?

Metrics Server itself doesn't calculate any metrics, it aggregates values exposed by Kubelet and exposes them in API
to be used for autoscaling. For any problem with metric values please contact SIG-Node.

### How often is metrics server released?

There is no hard release schedule. A release is done after an important feature is implemented or upon request.

### Can I run more than one instance of metrics-server?

Yes, more than one instance can be deployed for High Availability. Each instance scrapes all nodes to collect metrics. Without aggregator routing, the apiserver typically talks to a single Metrics Server endpoint; with `--enable-aggregator-routing=true` on the kube-apiserver, requests can be load balanced across instances. [More Info.](./README.md#high-availability)

### How to run metrics-server securely?

Suggested configuration:

- Cluster with [RBAC] enabled
- Kubelet [read-only port] port disabled
- Validate kubelet certificate by mounting CA file and providing `--kubelet-certificate-authority` flag to metrics server
- Avoid passing insecure flags to metrics server (`--deprecated-kubelet-completely-insecure`, `--kubelet-insecure-tls`)
- Consider using your own certificates (`--tls-cert-file`, `--tls-private-key-file`)

### How to secure the connection between kube-apiserver and Metrics Server?

The `metrics.k8s.io` API is served through the Kubernetes API aggregation layer: the
kube-apiserver proxies aggregated API requests to Metrics Server over TLS. By default,
the Metrics Server APIServices (`v1.metrics.k8s.io` and `v1beta1.metrics.k8s.io`) are
configured with `insecureSkipTLSVerify: true` (see `manifests/base/apiservice.yaml`),
which tells the apiserver to skip verification of the certificate Metrics Server presents.
To secure this connection:

1. Serve Metrics Server over TLS with a certificate (`--tls-cert-file` and
   `--tls-private-key-file`).
2. Set `insecureSkipTLSVerify: false` on each Metrics Server APIService you install.
3. Provide the PEM-encoded CA certificate (the certificate that signed Metrics Server's
   serving certificate) in the APIService `caBundle` field so the apiserver can verify it.

When installing with Helm, set `apiService.insecureSkipTLSVerify: false` and
`apiService.caBundle` to the PEM-encoded CA certificate, and choose a `tls.type`
(`metrics-server`, `helm`, `cert-manager`, or `existingSecret`) to provision the
serving certificate.

### How to run metric-server on different architecture?

Starting from `v0.3.7` docker image `registry.k8s.io/metrics-server/metrics-server` should support multiple architectures via Manifests List.
List of supported architectures: `amd64`, `arm`, `arm64`, `ppc64le`, `s390x`.

### What Kubernetes versions are supported?

See the [Compatibility Matrix](./README.md#compatibility-matrix). Metrics Server is continuously tested against the latest three Kubernetes minor versions used in this repository's e2e jobs.

### How is resource utilization calculated?

Metrics server doesn't provide resource utilization metrics (e.g. percent of CPU used).
Utilization presented by `kubectl top` and HPA is calculated client side based on pod resource requests or node capacity.

### How to autoscale Metrics Server?

Metrics server scales linearly vertically according to the number of nodes and pods in a cluster. This can be automated using [addon-resizer].

### Can I get other metrics beside CPU/Memory using Metrics Server?

No, metrics server was designed to provide metrics for [resource metrics pipeline] used for autoscaling.

### How large can clusters be?

Metrics Server was tested to run within clusters up to 5000 nodes with an average pod density of 30 pods per node.

### How often metrics are scraped?

The Metrics Server binary defaults to a 60s metric resolution (`--metric-resolution`).
Official manifests and the Helm chart set `--metric-resolution=15s`. Values below 15s
are not recommended, as that is the resolution of metrics calculated by Kubelet.

[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[read-only port]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/#options
[addon-resizer]: https://github.com/kubernetes/autoscaler/tree/master/addon-resizer
[resource metrics pipeline]: https://kubernetes.io/docs/tasks/debug-application-cluster/resource-metrics-pipeline/
