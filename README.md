# Kubernetes Metrics Server

Metrics server is one of the components in core metrics pipeline described in [Kubernetes monitoring architecture].
Metrics server is responsible for collecting resource metrics from kubelets and exposing them in Kubernetes Apiserver
through [Metrics API]. Main consumers of those metrics are `kubectl top`, [HPA] and [VPA]. Metric server stores only the
latest values of metrics needed for core metrics pipeline (CPU, Memory) and is not responsible for forwarding metrics
to third-party destinations.

## Design

The detailed design of the project can be found in the following docs:

- [Metrics API design]
- [Metrics Server design]

## Requirements

Metrics server has particular requirements on cluster and network configuration that is not default for all cluster distributions. Please ensure that your cluster distribution supports those requirements before using metrics server:
* Metrics server needs to be reachable from kube-apiserver ([Configuring master to cluster communication])
* Kube-apiserver should be correctly configured to enable aggregation layer ([How to configure aggregation layer])
* Nodes need to have kubelet authorization configured and match metrics-server configuration ([How to configure kubelet authorization])
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
$ kubectl apply -f deploy/kubernetes/
```

You can also use this helm chart to deploy the metric-server in your cluster (This isn't supported by the metrics-server maintainers): https://github.com/helm/charts/tree/master/stable/metrics-server

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
[HPA]: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
[VPA]: https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler
[Frequently Asked Questions]: FAQ.md