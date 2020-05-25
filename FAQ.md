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

Starting from `v0.3.7` docker image `k8s.gcr.io/metrics-server` should support multiple architectures via Manifests List.
List of supported architectures: `amd64`, `arm`, `arm64`, `ppc64le`, `s390x`.

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

## Known issues

#### Incorrectly configured front-proxy certificate

Metrics Server needs to validate requests coming from kube-apiserver. You can recognize problems with front-proxy certificate configuration if you observe line below in your logs:
```
E0524 01:37:36.055326       1 authentication.go:65] Unable to authenticate the request due to an error: x509: certificate signed by unknown authority
```

To fix this problem you need to provide kube-apiserver proxy-client CA to Metrics Server under `--requestheader-client-ca-file` flag. You can read more about this flag in [Authenticating Proxy](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#authenticating-proxy)


For cluster created by kubeadm:

1. Check kube-apiserver front-proxy args(/etc/kubernetes/manifests/kube-apiserver.yaml)

```
- --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt
- --proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client.key
```

2. Prepare front-proxy-ca for Metrics Server

```
kubectl -nkube-system create configmap front-proxy-ca --from-file=front-proxy-ca.crt=/etc/kubernetes/pki/front-proxy-ca.crt -o yaml | kubectl -nkube-system replace configmap front-proxy-ca -f -
```

3. Configure your Metrics Server deployment:

```
      - args:
        - --requestheader-client-ca-file=/ca/front-proxy-ca.crt // ADD THIS!
        - --cert-dir=/tmp
        - --secure-port=4443
        - --kubelet-insecure-tls // ignore validate kubelet x509
        - --kubelet-preferred-address-types=InternalIP // using InternalIP to connect kubelet

        volumeMounts:
        - mountPath: /tmp
          name: tmp-dir
        - mountPath: /ca // ADD THIS!
          name: ca-dir

      volumes:
      - emptyDir: {}
        name: tmp-dir
      - configMap: // ADD THIS!
          defaultMode: 420
          name: front-proxy-ca
        name: ca-dir
```

#### Network problems

Metrics server needs to contact all nodes in cluster to collect metrics. Problems with network would can be recognized by following symptoms:

When running `kubectl top nodes` we get partial information. For example results like:
```
NAME         CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%     
k8s-node01   59m          5%     1023Mi          26%         
k8s-master   <unknown>                           <unknown>               <unknown>               <unknown>               
k8s-node02   <unknown>                           <unknown>               <unknown>               <unknown>         
```

In logs we will see problems with fetching metrics from Kubelets, in particular errors will include `dial tcp IP(or hostname):10250: i/o timeout`

kubectl logs -n kube-system -l k8s-app=metrics-server --container metrics-server

```
unable to fully collect metrics: [unable to fully scrape metrics from source kubelet_summary:k8s-master: unable to fetch metrics from Kubelet k8s-master
(192.168.17.150): Get https://192.168.17.150:10250/stats/summary?only_cpu_and_memory=true: dial tcp 192.168.17.150:10250: i/o timeout, 
unable to fully scrape metrics from source kubelet_summary:k8s-node02: unable to fetch metrics from Kubelet k8s-node02 (192.168.17.152):
Get https://192.168.17.152:10250/stats/summary?only_cpu_and_memory=true: dial tcp 192.168.17.152:10250: i/o timeout
```

Known solutions:
* **[Calico]** Check whether the value of `CALICO_IPV4POOL_CIDR` in the calico.yaml conflicts with the local physical network segment. The default: `192.168.0.0/16`.

See [Kubernetes in Calico] for more details.

[Meaning of CPU]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
[Meaning of memory]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
[RBAC]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[read-only port]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/#options
[addon-resizer]: https://github.com/kubernetes/autoscaler/tree/master/addon-resizer
[Kubernetes in Calico]: https://docs.projectcalico.org/getting-started/kubernetes/quickstart
