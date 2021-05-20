# Kubernetes Metrics Server

Metrics Server is a scalable, efficient source of container resource metrics for Kubernetes
built-in autoscaling pipelines.

Metrics Server collects resource metrics from Kubelets and exposes them in Kubernetes apiserver through [Metrics API] 
for use by [Horizontal Pod Autoscaler] and [Vertical Pod Autoscaler]. Metrics API can also be accessed by `kubectl top`,
making it easier to debug autoscaling pipelines.

Metrics Server is not meant for non-autoscaling purposes. For example, don't use it to forward metrics to monitoring solutions, or as a source of monitoring solution metrics.

Metrics Server offers:
- A single deployment that works on most clusters (see [Requirements](#requirements))
- Fast autoscaling, collecting metrics every 15 seconds.
- Resource efficiency, using 1 mili core of CPU and 2 MB of memory for each node in a cluster.
- Scalable support up to 5,000 node clusters.

[Metrics API]: https://github.com/kubernetes/metrics
[Horizontal Pod Autoscaler]: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
[Vertical Pod Autoscaler]: https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler/

## Use cases

You can use Metrics Server for:
- CPU/Memory based horizontal autoscaling (learn more about [Horizontal Autoscaling])
- Automatically adjusting/suggesting resources needed by containers (learn more about [Vertical Autoscaling])

Don't use Metrics Server when you need:
- Non-Kubernetes clusters
- An accurate source of resource usage metrics
- Horizontal autoscaling based on other resources than CPU/Memory

For unsupported use cases, check out full monitoring solutions like Prometheus.

[Horizontal Autoscaling]: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
[Vertical Autoscaling]: https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler/

## Requirements

Metrics Server has specific requirements for cluster and network configuration. These requirements aren't the default for all cluster
distributions. Please ensure that your cluster distribution supports these requirements before using Metrics Server:
- Metrics Server must be [reachable from kube-apiserver] by container IP address (or node IP if hostNetwork is enabled).
- The kube-apiserver must [enable an aggregation layer].
- Nodes must have Webhook [authentication and authorization] enabled.
- Kubelet certificate needs to be signed by cluster Certificate Authority (or disable certificate validation by passing `--kubelet-insecure-tls` to Metrics Server)
- Container runtime must implement a [container metrics RPCs] (or have [cAdvisor] support)

[reachable from kube-apiserver]: https://kubernetes.io/docs/concepts/architecture/master-node-communication/#master-to-cluster
[enable an aggregation layer]: https://kubernetes.io/docs/tasks/access-kubernetes-api/configure-aggregation-layer/
[authentication and authorization]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-authentication-authorization/
[container metrics RPCs]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-node/cri-container-stats.md
[cAdvisor]: https://github.com/google/cadvisor

## Installation

Latest Metrics Server release can be installed by running:

```shell
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

Installation instructions for previous releases can be found in [Metrics Server releases].

Compatibility matrix:

Metrics Server | Metrics API group/version | Supported Kubernetes version
---------------|---------------------------|-----------------------------
0.5.x          | `metrics.k8s.io/v1beta1`  | *1.8+
0.4.x          | `metrics.k8s.io/v1beta1`  | *1.8+
0.3.x          | `metrics.k8s.io/v1beta1`  | 1.8-1.21

*For <1.16 requires passing `--authorization-always-allow-paths=/livez,/readyz` command line flag

[Metrics Server releases]: https://github.com/kubernetes-sigs/metrics-server/releases

## Security context

Metrics Server requires the `CAP_NET_BIND_SERVICE` capability in order to bind to a privileged ports as non-root.
If you are running Metrics Server in an environment that uses PSPs or other mechanisms to restrict pod capabilities, ensure that Metrics Server is allowed
to use this capability.
This applies even if you use the `--secure-port` flag to change the port that Metrics Server binds to to a non-privileged port.

## Scaling

Starting from v0.5.0 Metrics Server comes with default resource requests that should guarantee good performance for most cluster configurations up to 100 nodes:

* 100m core of CPU
* 200MiB of memory

Metrics Server resource usage depends on multiple independent dimensions, creating a [Scalability Envelope].
Default Metrics Server configuration should work in clusters that don't exceed any of the thresholds listed below:

Quantity               | Namespace threshold | Cluster threshold
-----------------------|---------------------|------------------
#Nodes                 | n/a                 | 100
#Pods per node         | 70                  | 70
#Deployments with HPAs | 100                 | 100

Resources can be adjusted proportionally based on number of nodes in the cluster.
For clusters of more than 100 nodes, allocate additionally:
* 1m core per node
* 2MiB memory per node

You can use the same approach to lower resource requests, but there is a boundary
where this may impact other scalability dimensions like maximum number of pods per node.

[Scalability Envelope]: https://github.com/kubernetes/community/blob/master/sig-scalability/configs-and-limits/thresholds.md

### Configuration 

Depending on your cluster setup, you may also need to change flags passed to the Metrics Server container.
Most useful flags:
- `--kubelet-preferred-address-types` - The priority of node address types used when determining an address for connecting to a particular node (default [Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP])
- `--kubelet-insecure-tls` - Do not verify the CA of serving certificates presented by Kubelets. For testing purposes only.
- `--requestheader-client-ca-file` - Specify a root certificate bundle for verifying client certificates on incoming requests.

You can get a full list of Metrics Server configuration flags by running:

```shell
docker run --rm k8s.gcr.io/metrics-server/metrics-server:v0.5.0 --help
```

#### Helm Chart

This [Helm chart](https://github.com/helm/charts/tree/master/stable/metrics-server) can deploy the metric-server service in your cluster.

Note: This Helm chart isn't supported by Metrics Server maintainers.

## Design

Metrics Server is a component in the core metrics pipeline described in [Kubernetes monitoring architecture].

For more information, see:
- [Metrics API design]
- [Metrics Server design]

[Kubernetes monitoring architecture]: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/monitoring_architecture.md
[Metrics API design]: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md
[Metrics Server design]: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/metrics-server.md

## Have a question?

Before posting an issue, first checkout [Frequently Asked Questions] and [Known Issues].

[Frequently Asked Questions]: FAQ.md
[Known Issues]: KNOWN_ISSUES.md

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page].

You can reach the maintainers of this project at:

- [Slack channel]
- [Mailing list]

This project is maintained by [SIG Instrumentation]

[community page]: http://kubernetes.io/community/
[Slack channel]: https://kubernetes.slack.com/messages/sig-instrumentation
[Mailing list]: https://groups.google.com/forum/#!forum/kubernetes-sig-instrumentation
[SIG Instrumentation]: https://github.com/kubernetes/community/tree/master/sig-instrumentation

### Development

Recommended tools for local development:
* [Kind](https://kind.sigs.k8s.io/)
* [Skaffold](https://skaffold.dev/)

To start local development just run:
```
kind create cluster
skaffold dev
```

To execute e2e tests run:
```
go test test/e2e_test.go -v -count=1
```

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct].

[Kubernetes Code of Conduct]: code-of-conduct.md
