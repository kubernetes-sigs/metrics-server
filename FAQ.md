## FAQ

#### What metrics are exposed by metrics server?

Metrics server collects resource usage metrics needed for autoscaling: CPU & Memory.
Metric values use standard kubernetes units (`m`, `Ki`), same as those used to
define pod requests and limits (Read more [Meaning of CPU], [Meaning of memory])
Metrics server itself is not responsible for calculating metric values, this is done by Kubelet.

#### When metrics server is released?

There is no hard release schedule. Release is done after important feature is implemented or upon request.

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

There are officially built images for `amd64`, `arm`, `arm64`, `ppc64le`, `s390x`. Please update manifests to use specific image e.g. `k8s.gcr.io/metrics-server-s390x:v0.3.6`

#### What Kubernetes versions are supported?

Metrics server is tested against last 3 Kubernetes versions.

#### How resource utilization is calculated?

Metrics server doesn't provide resource utilization metric (e.g. percent of CPU used).
Kubectl top and HPA calculate those values by themselves based on pod resource requests or node capacity.

#### How to autoscale Metrics Server?

Metrics server scales linearly vertically to number of nodes and pods in cluster. This can be automated using [addon-resizer].

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

[Meaning of CPU]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
[Meaning of memory]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[read-only port]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/#options
[addon-resizer]: https://github.com/kubernetes/autoscaler/tree/master/addon-resizer