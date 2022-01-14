# Kubernetes Metrics Server

Metrics Server is a scalable, efficient source of container resource metrics for Kubernetes
built-in autoscaling pipelines.

Metrics Server collects resource metrics from Kubelets and exposes them in Kubernetes apiserver through [Metrics API] 
for use by [Horizontal Pod Autoscaler] and [Vertical Pod Autoscaler]. Metrics API can also be accessed by `kubectl top`,
making it easier to debug autoscaling pipelines.

Metrics Server is not meant for non-autoscaling purposes. For example, don't use it to forward metrics to monitoring solutions, or as a source of monitoring solution metrics. In such cases please collect metrics from Kubelet `/metrics/resource` endpoint directly.

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

Metrics Server can be installed either directly from YAML manifest or via the official [Helm chart](https://artifacthub.io/packages/helm/metrics-server/metrics-server). To install the latest Metrics Server release from the _components.yaml_ manifest, run the following command.

```shell
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

Installation instructions for previous releases can be found in [Metrics Server releases](https://github.com/kubernetes-sigs/metrics-server/releases).

### Compatibility Matrix

Metrics Server | Metrics API group/version | Supported Kubernetes version
---------------|---------------------------|-----------------------------
0.6.x          | `metrics.k8s.io/v1beta1`  | *1.19+
0.5.x          | `metrics.k8s.io/v1beta1`  | *1.8+
0.4.x          | `metrics.k8s.io/v1beta1`  | *1.8+
0.3.x          | `metrics.k8s.io/v1beta1`  | 1.8-1.21

*Kubernetes versions lower than v1.16 require passing the `--authorization-always-allow-paths=/livez,/readyz` command line flag

### High Availability

Metrics Server can be installed in high availability mode directly from a YAML manifest or via the official [Helm chart](https://artifacthub.io/packages/helm/metrics-server/metrics-server) by setting the `replicas` value greater than `1`. To install the latest Metrics Server release in high availability mode from the  _high-availability.yaml_ manifest, run the following command.

```shell
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/high-availability.yaml
```

Note that this configuration **requires** having a cluster with at least 2 nodes on which Metrics Server can be scheduled.

Also, to maximize the efficiency of this highly available configuration, it is **recommended** to add the `--enable-aggregator-routing=true` CLI flag to the kube-apiserver so that requests sent to Metrics Server are load balanced between the 2 instances.

### Helm Chart

The [Helm chart](https://artifacthub.io/packages/helm/metrics-server/metrics-server) is maintained as an additional component within this repo and released into a chart repository backed on the `gh-pages` branch. A new version of the chart will be released for each Metrics Server release and can also be released independently if there is a need. The chart on the `master` branch shouldn't be referenced directly as it might contain modifications since it was last released, to view the chart code use the chart release tag.

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

A potentially outdated list of flags found via the above command is below:

````txt
Usage:
   [flags]

Flags:
      --alsologtostderr                                         log to standard error as well as files
      --authentication-kubeconfig string                        kubeconfig file pointing at the 'core' kubernetes server with enough rights to create tokenaccessreviews.authentication.k8s.io.
      --authentication-skip-lookup                              If false, the authentication-kubeconfig will be used to lookup missing authentication configuration from the cluster.
      --authentication-token-webhook-cache-ttl duration         The duration to cache responses from the webhook token authenticator. (default 10s)
      --authentication-tolerate-lookup-failure                  If true, failures to look up missing authentication configuration from the cluster are not considered fatal. Note that this can result in authentication that treats all requests as anonymous.
      --authorization-always-allow-paths strings                A list of HTTP paths to skip during authorization, i.e. these are authorized without contacting the 'core' kubernetes server.
      --authorization-kubeconfig string                         kubeconfig file pointing at the 'core' kubernetes server with enough rights to create subjectaccessreviews.authorization.k8s.io.
      --authorization-webhook-cache-authorized-ttl duration     The duration to cache 'authorized' responses from the webhook authorizer. (default 10s)
      --authorization-webhook-cache-unauthorized-ttl duration   The duration to cache 'unauthorized' responses from the webhook authorizer. (default 10s)
      --bind-address ip                                         The IP address on which to listen for the --secure-port port. The associated interface(s) must be reachable by the rest of the cluster, and by CLI/web clients. If blank, all interfaces will be used (0.0.0.0 for all IPv4 interfaces and :: for all IPv6 interfaces). (default 0.0.0.0)
      --cert-dir string                                         The directory where the TLS certs are located. If --tls-cert-file and --tls-private-key-file are provided, this flag will be ignored. (default "apiserver.local.config/certificates")
      --client-ca-file string                                   If set, any request presenting a client certificate signed by one of the authorities in the client-ca-file is authenticated with an identity corresponding to the CommonName of the client certificate.
      --contention-profiling                                    Enable lock contention profiling, if profiling is enabled
  -h, --help                                                    help for this command
      --http2-max-streams-per-connection int                    The limit that the server gives to clients for the maximum number of streams in an HTTP/2 connection. Zero means to use golang's default.
      --kubeconfig string                                       The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)
      --kubelet-certificate-authority string                    Path to the CA to use to validate the Kubelet's serving certificates.
      --kubelet-insecure-tls                                    Do not verify CA of serving certificates presented by Kubelets.  For testing purposes only.
      --kubelet-port int                                        The port to use to connect to Kubelets. (default 10250)
      --kubelet-preferred-address-types strings                 The priority of node address types to use when determining which address to use to connect to a particular node (default [Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP])
      --log-flush-frequency duration                            Maximum number of seconds between log flushes (default 5s)
      --log_backtrace_at traceLocation                          when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                                          If non-empty, write log files in this directory
      --log_file string                                         If non-empty, use this log file
      --logtostderr                                             log to standard error instead of files (default true)
      --metric-resolution duration                              The resolution at which metrics-server will retain metrics. (default 1m0s)
      --profiling                                               Enable profiling via web interface host:port/debug/pprof/ (default true)
      --requestheader-allowed-names strings                     List of client certificate common names to allow to provide usernames in headers specified by --requestheader-username-headers. If empty, any client certificate validated by the authorities in --requestheader-client-ca-file is allowed.
      --requestheader-client-ca-file string                     Root certificate bundle to use to verify client certificates on incoming requests before trusting usernames in headers specified by --requestheader-username-headers. WARNING: generally do not depend on authorization being already done for incoming requests.
      --requestheader-extra-headers-prefix strings              List of request header prefixes to inspect. X-Remote-Extra- is suggested. (default [x-remote-extra-])
      --requestheader-group-headers strings                     List of request headers to inspect for groups. X-Remote-Group is suggested. (default [x-remote-group])
      --requestheader-username-headers strings                  List of request headers to inspect for usernames. X-Remote-User is common. (default [x-remote-user])
      --secure-port int                                         The port on which to serve HTTPS with authentication and authorization.If 0, don't serve HTTPS at all. (default 443)
      --skip_headers                                            If true, avoid header prefixes in the log messages
      --stderrthreshold severity                                logs at or above this threshold go to stderr
      --tls-cert-file string                                    File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert). If HTTPS serving is enabled, and --tls-cert-file and --tls-private-key-file are not provided, a self-signed certificate and key are generated for the public address and saved to the directory specified by --cert-dir.
      --tls-cipher-suites strings                               Comma-separated list of cipher suites for the server. If omitted, the default Go cipher suites will be use.  Possible values: TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_RC4_128_SHA,TLS_RSA_WITH_3DES_EDE_CBC_SHA,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_128_CBC_SHA256,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_RC4_128_SHA
      --tls-min-version string                                  Minimum TLS version supported. Possible values: VersionTLS10, VersionTLS11, VersionTLS12
      --tls-private-key-file string                             File containing the default x509 private key matching --tls-cert-file.
      --tls-sni-cert-key namedCertKey                           A pair of x509 certificate and private key file paths, optionally suffixed with a list of domain patterns which are fully qualified domain names, possibly with prefixed wildcard segments. If no domain patterns are provided, the names of the certificate are extracted. Non-wildcard matches trump over wildcard matches, explicit domain patterns trump over extracted names. For multiple key/certificate pairs, use the --tls-sni-cert-key multiple times. Examples: "example.crt,example.key" or "foo.crt,foo.key:*.foo.com,foo.com". (default [])
  -v, --v Level                                                 number for the log level verbosity
      --vmodule moduleSpec                                      comma-separated list of pattern=N settings for file-filtered logging
```

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

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct].

[Kubernetes Code of Conduct]: code-of-conduct.md
