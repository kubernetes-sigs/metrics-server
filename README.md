# Kubernetes Metrics Server

Metrics Server is a scalable, efficient source of container resource metrics for Kubernetes
built-in autoscaling pipelines.

Metrics Server collects resource metrics from Kubelets and exposes them in Kubernetes apiserver through [Metrics API] 
for use by [Horizontal Pod Autoscaler] and [Vertical Pod Autoscaler]. Metrics API can also be accessed by `kubectl top`,
making it easier to debug autoscaling pipelines.

Metrics Server is not meant for non-autoscaling purposes. For example, don't use it to forward metrics to monitoring solutions, or as a source of monitoring solution metrics.

Metrics Server offers:
- A single deployment that works on most clusters (see [Requirements](#requirements))
- Scalable support up to 5,000 node clusters
- Resource efficiency: Metrics Server uses 1m core of CPU and 3 MB of memory per node

## Use cases

You can use Metrics Server for:
- CPU/Memory based horizontal autoscaling (learn more about [Horizontal Pod Autoscaler])
- Automatically adjusting/suggesting resources needed by containers (learn more about [Vertical Pod Autoscaler])

Don't use Metrics Server when you need:
- Non-Kubernetes clusters
- An accurate source of resource usage metrics
- Horizontal autoscaling based on other resources than CPU/Memory

For unsupported use cases, check out full monitoring solutions like Prometheus.

## Requirements

Metrics Server has specific requirements for cluster and network configuration. These requirements aren't the default for all cluster
distributions. Please ensure that your cluster distribution supports these requirements before using Metrics Server:
- Metrics Server must be [reachable from kube-apiserver]
- The kube-apiserver must be correctly configured to [enable an aggregation layer]
- Nodes must have [kubelet authorization] configured to match Metrics Server configuration
- Container runtime must implement a [container metrics RPCs]

## Installation

Latest Metrics Server release can be installed by running:

```shell
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

Installation instructions for previous releases can be found in [Metrics Server releases].

WARNING: You should no longer use manifests from `master` branch (previously available in `deploy/kubernetes` directory).
They are now meant solely for development.

Compatibility matrix:

Metrics Server | Metrics API group/version | Supported Kubernetes version
---------------|---------------------------|-----------------------------
0.4.x          | `metrics.k8s.io/v1beta1`  | 1.8+
0.3.x          | `metrics.k8s.io/v1beta1`  | 1.8+

## Scaling

Starting from v0.5.0 Metrics Server comes with default resource requests that should guarantee good performance for most cluster configurations up to 100 nodes:

* 100m core of CPU
* 300MiB of memory

If neeed resources can be scaled proportionally based on the number of nodes (minimum of 10 nodes).
For example, clusters up to 10 nodes can request 10m core CPU and 30MiB memory.
Please remember that lowering resources will also proportionally reduce other thresholds like maximum number of autoscaled deployments (down to 10 for 10 node clusters).

Metrics Server resource usage depends on multiple independent dimensions, creating a [Scalability Envelope].
Default Metrics Server configuration should work in clusters that don't exceed any of the thresholds listed below:

[Scalability Envelope]: https://github.com/kubernetes/community/blob/master/sig-scalability/configs-and-limits/thresholds.md

Quantity            | Namespace threshold | Cluster threshold
--------------------|---------------------|------------------
# Nodes             | n/a                 | 100
# Pods              | 7000                | 7000
# Deployments + HPA | 100                 | 100

For clusters of more than 100 nodes, allocate additionally:
* 1m core per node
* 3MiB memory per node

### Configuration 

Depending on your cluster setup, you may also need to change flags passed to the Metrics Server container.
Most useful flags:
- `--kubelet-preferred-address-types` - The priority of node address types used when determining an address for connecting to a particular node (default [Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP])
- `--kubelet-insecure-tls` - Do not verify the CA of serving certificates presented by Kubelets. For testing purposes only.
- `--requestheader-client-ca-file` - Specify a root certificate bundle for verifying client certificates on incoming requests.

You can get a full list of Metrics Server configuration flags by running:

```shell
docker run --rm k8s.gcr.io/metrics-server/metrics-server:v0.3.7 --help
```

#### Helm Chart

This [Helm chart](https://github.com/helm/charts/tree/master/stable/metrics-server) can deploy the metric-server service in your cluster.

Note: This Helm chart isn't supported by Metrics Server maintainers.

## Design

Metrics Server is a component in the core metrics pipeline described in [Kubernetes monitoring architecture].

For more information, see:
- [Metrics API design]
- [Metrics Server design]

## Have a question?

Before posting it an issue, first checkout [Frequently Asked Questions].

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page].

You can reach the maintainers of this project at:

- [Slack channel]
- [Mailing list]

This project is maintained by [SIG Instrumentation]

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct].

[Kubernetes monitoring architecture]: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/monitoring_architecture.md
[Metrics API]: https://github.com/kubernetes/metrics
[Metrics API design]: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md
[Metrics Server design]: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/metrics-server.md
[reachable from kube-apiserver]: https://kubernetes.io/docs/concepts/architecture/master-node-communication/#master-to-cluster
[enable an aggregation layer]: https://kubernetes.io/docs/tasks/access-kubernetes-api/configure-aggregation-layer/
[kubelet authorization]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-authentication-authorization/
[container metrics RPCs]:https://github.com/kubernetes/community/blob/master/contributors/devel/sig-node/cri-container-stats.md
[SIG Instrumentation]: https://github.com/kubernetes/community/tree/master/sig-instrumentation
[Slack channel]: https://kubernetes.slack.com/messages/sig-instrumentation
[Mailing list]: https://groups.google.com/forum/#!forum/kubernetes-sig-instrumentation
[Kubernetes Code of Conduct]: code-of-conduct.md
[community page]: http://kubernetes.io/community/
[Horizontal Pod Autoscaler]: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
[Vertical Pod Autoscaler]: https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler
[Frequently Asked Questions]: FAQ.md
[Metrics Server releases]: https://github.com/kubernetes-sigs/metrics-server/releases
