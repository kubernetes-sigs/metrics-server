# Kubernetes Metrics Server

Metrics Server is a simple, scalable, resource efficient source of container resource metrics needed for Kubernetes
built-in autoscaling pipelines.

Metrics Server collects resource metrics from Kubelets and expose them in Kubernetes apiserver through [Metrics API] to
be used by [Horizontal Pod Autoscaler] and [Vertical Pod Autoscaler]. Metrics API can also be accessed by `kubectl top`
allowing for easier debugging of autoscaling pipelines.

Metric Server is not meant for non-autoscaling purposes, it is not responsible for forwarding metrics to monitoring
solutions nor should be used as source of metrics for them.

Metrics Server good points:
- Simple - single deployment that should out-of-the box on most clusters (see [Requirements](#requirements) section)
- Scalable - supports up to 5k node clusters
- Resource efficient - uses 0.5m core of CPU and 4 MB of memory per node

## Use cases

Metrics Server **can** be used when you want:
- CPU/Memory based horizontal autoscaling (learn more about [Horizontal Pod Autoscaler])
- Automatic adjusting/suggesting resources needed by containers (learn more about [Vertical Pod Autoscaler])

Metrics Server **should not** be used when you need:
- Non-Kubernetes clusters
- Accurate source of resource usage metrics
- Horizontal autoscaling based on other resources then CPU/Memory

For unsupported use cases please checkout full monitoring solutions like Prometheus.

## Requirements

Metrics server has particular requirements on cluster and network configuration that is not default for all cluster
distributions. Please ensure that your cluster distribution supports those requirements before using Metrics Server:
- Metrics server needs to be reachable from kube-apiserver ([Configuring master to cluster communication])
- Kube-apiserver should be correctly configured to enable aggregation layer ([How to configure aggregation layer])
- Nodes need to have kubelet authorization configured and match Metrics Server configuration ([How to configure kubelet authorization])
- Pod/Node metrics need to be exposed by Kubelet by Summary API

## Deployment

Compatibility matrix:

Metrics Server | Metrics API group/version | Supported Kubernetes version
---------------|---------------------------|-----------------------------
0.3.x          | `metrics.k8s.io/v1beta1`  | 1.8+


In order to deploy Metrics Server in your cluster run the following command:

```console
$ kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.3.6/components.yaml
```

Depending on your cluster configuration you may also need to change flags passed to Metrics Server container.
Most useful flags:
- `--kubelet-preferred-address-types` - The priority of node address types to use when determining which address to use
 to connect to a particular node (default [Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP])
- `--kubelet-insecure-tls` - Do not verify CA of serving certificates presented by Kubelets.  For testing purposes only.
- `--requestheader-client-ca-file` Root certificate bundle to use to verify client certificates on incoming requests
before trusting usernames in headers specified by --requestheader-username-headers. WARNING: generally do not depend on
authorization being already done for incoming requests

You can get full list of Metrics Server configuration flags by running:

```
docker run --rm k8s.gcr.io/metrics-server:v0.3.7 --help
```

You can also use this helm chart to deploy the metric-server in your cluster (This isn't supported by the Metrics Server
 maintainers): https://github.com/helm/charts/tree/master/stable/metrics-server

## Design

Metrics server is one of the components in core metrics pipeline described in [Kubernetes monitoring architecture].

The detailed design of the project can be found in the following docs:
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
[Configuring master to cluster communication]: https://kubernetes.io/docs/concepts/architecture/master-node-communication/#master-to-cluster
[How to configure aggregation layer]: https://kubernetes.io/docs/tasks/access-kubernetes-api/configure-aggregation-layer/
[How to configure kubelet authorization]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet-authentication-authorization/
[SIG Instrumentation]: https://github.com/kubernetes/community/tree/master/sig-instrumentation
[Slack channel]: https://kubernetes.slack.com/messages/sig-instrumentation
[Mailing list]: https://groups.google.com/forum/#!forum/kubernetes-sig-instrumentation
[Kubernetes Code of Conduct]: code-of-conduct.md
[community page]: http://kubernetes.io/community/
[Horizontal Pod Autoscaler]: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
[Vertical Pod Autoscaler]: https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler
[Frequently Asked Questions]: FAQ.md