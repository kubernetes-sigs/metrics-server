# Kubernetes Metrics Server

## User guide

You can find the user guide in
[the official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug-application-cluster/resource-metrics-pipeline/).

## Design

The detailed design of the project can be found in the following docs:

- [Metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md)
- [Metrics Server](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/metrics-server.md)

For the broader view of monitoring in Kubernetes take a look into
[Monitoring architecture](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/monitoring_architecture.md)

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

If you want to test `metric-server` in a `minikube` cluster, please follow the steps below:

```console
$ minikube version
minikube version: v1.2.0

# disable the metrics-server addon for minikube in case it was enabled, because it installs the metric-server@v0.2.1
$ minikube addons disable metrics-server

# now start a new minikube
$ minikube delete; minikube start --extra-config=kubelet.authentication-token-webhook=true
🔥  Deleting "minikube" from virtualbox ...
💔  The "minikube" cluster has been deleted.
😄  minikube v1.2.0 on linux (amd64)
🔥  Creating virtualbox VM (CPUs=2, Memory=2048MB, Disk=20000MB) ...
🐳  Configuring environment for Kubernetes v1.15.0 on Docker 18.09.6
    ▪ kubelet.authentication-token-webhook=true
🚜  Pulling images ...
🚀  Launching Kubernetes ...
⌛  Verifying: apiserver proxy etcd scheduler controller dns
🏄  Done! kubectl is now configured to use "minikube"

# deploy the latest metric-server
$ kubectl create -f deploy/kubernetes/
clusterrole.rbac.authorization.k8s.io/system:aggregated-metrics-reader created
clusterrolebinding.rbac.authorization.k8s.io/metrics-server:system:auth-delegator created
rolebinding.rbac.authorization.k8s.io/metrics-server-auth-reader created
apiservice.apiregistration.k8s.io/v1beta1.metrics.k8s.io created
serviceaccount/metrics-server created
deployment.extensions/metrics-server created
service/metrics-server created
clusterrole.rbac.authorization.k8s.io/system:metrics-server created
clusterrolebinding.rbac.authorization.k8s.io/system:metrics-server created

# edit metric-server deployment to add the flags
# args:
# - --kubelet-insecure-tls
# - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
$ kubectl edit deploy -n kube-system metrics-server
```
![minikube-metric-server-args](deploy/minikube/metric-server-args.png)

## Known issues

#### Metrics Server takes long to start

I0911 14:01:46.782718       1 serving.go:312] Generated self-signed cert (apiserver.local.config/certificates/apiserver.crt, apiserver.local.config/certificates/apiserver.key)
I0911 14:01:47.315002       1 secure_serving.go:116] Serving securely on [::]:8443

#### My HPA doesn't work

Read HPA events
Check kubectl top
Read controller-manager logs
Check RBAC
Contact the HPA people!

#### Kube dashboard doesn't work

#### Metrics API Unavailable

`Error from server (ServiceUnavailable): the server is currently unable to handle the request (get pods.metrics.k8s.io)`
`Error from server (ServiceUnavailable): the server is currently unable to handle the request (get nodes.metrics.k8s.io)`

kubectl describe apiservice v1beta1.metrics.k8s.io

Condition
Get https://10.110.144.114:443/apis/metrics.k8s.io/v1beta1: dial tcp 10.110.144.114:443: connect: no route to host
Get https://10.108.222.161:443: net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
Get https://10.12.0.10:443: No SSH tunnels currently open. Were the targets able to accept an ssh-key for user "gke-386b15b70ba6e2f32e33"
Get https://172.17.111.146:443/apis/metrics.k8s.io/v1beta1: dial tcp 172.17.111.146:443: connect: no route to host

Apiserver logs
available_controller.go:353] v1beta1.metrics.k8s.io failed with: Get https://10.32.0.169:443: dial tcp 10.32.0.169:443: connect: no route to host


https://kubernetes.io/docs/tasks/access-kubernetes-api/configure-aggregation-layer/
https://kubernetes.io/docs/concepts/architecture/master-node-communication/#ssh-tunnels
Problem with network, check if kube-proxy is installed on master or use "--enable-aggregator-routing=true"
Consider running metrics-server in hostNetwork
Check if apiserver can reach node network

#### Container start
E1017 08:21:07.209911       1 manager.go:102] unable to fully collect metrics: unable to fully scrape metrics from source kubelet_summary:ip-192-168-64-66.eu-west-3.compute.internal: [unable to get CPU for container "redacted-pod-name-first" in pod prod/prod-redacted-pod-name-first-1571300460-ht5cp on node "ip-192-168-64-66.eu-west-3.compute.internal", discarding data: missing cpu usage metric, unable to get CPU for container "redacted-pod-name-second" in pod prod/prod-redacted-pod-name-second-1571300460-xdrqz on node "ip-192-168-64-66.eu-west-3.compute.internal", discarding data: missing cpu usage metric

I'm pretty sure CAdvisor responsible for those metrics will skip adding CPU metrics if it didn't have time to do housekeeping (every 15s). So if you have dynamic cluster (containers starting frequently) this message can happen often (metrics server asking about CPU for containers younger then 15s).

Logs "missing cpu usage metric" for longer running pods mean problem with Kubelet metrics.

#### Metrics Server crashlooping
Error: Get https://10.96.0.1:443/api/v1/namespaces/kube-system/configmaps/extension-apiserver-authentication: dial tcp 10.96.0.1:443: i/o timeout
Usage:


### VPA not working

#### No tmp directory

Error: error creating self-signed certificates: mkdir apiserver.local.config: read-only file system

#### No metrics!

E0102 17:49:34.604949       1 reststorage.go:160] unable to fetch pod metrics for pod kube-system/harbor-ui-rc-77f696b975-gbqjt: no metrics known for pod
E0204 19:19:41.128358       1 manager.go:111] unable to fully collect metrics: unable to fully scrape metrics from source kubelet_summary:master-node: unable to fetch metrics from Kubelet master-node (master-node): Get https://master-node:10250/stats/summary?only_cpu_and_memory=true: dial tcp: lookup master-node on 10.96.0.10:53: server misbehaving
0113 13:30:33.188732 1 reststorage.go:144] unable to fetch pod metrics for pod aps-serving/m-62da378b1ced488cbf6ae9bb2f52a363-bf6ccbffc-2g767: no metrics known for pod
E0113 13:30:38.449081 1 manager.go:102] unable to fully collect metrics: unable to fully scrape metrics from source kubelet_summary:192.168.7.121: [unable to get a valid timestamp for metric point for container "m-62da378b1ced488cbf6ae9bb2f52a363" in pod aps-serving/m-62da378b1ced488cbf6ae9bb2f52a363-bf6ccbffc-zng48 on node "192.168.7.121",
E1216 15:23:24.537445       1 manager.go:111] unable to fully collect metrics: [unable to fully scrape metrics from source kubelet_summary:workers-p90e: unable to fetch metrics from Kubelet workers-p90e (workers-p90e): Get https://workers-p90e:10250/stats/summary?only_cpu_and_memory=true: dial tcp: lookup workers-p90e on 10.245.0.10:53: no such host, unable to fully scrape metrics from source kubelet_summary:workers-p90g: unable to fetch metrics from Kubelet workers-p90g (workers-p90g): Get https://workers-p90g:10250/stats/summary?only_cpu_and_memory=true: dial tcp: lookup workers-p90g on 10.245.0.10:53: no such host, unable to fully scrape metrics from source kubelet_summary:workers-p90a: unable to fetch metrics from Kubelet workers-p90a (workers-p90a): Get https://workers-p90a:10250/stats/summary?only_cpu_and_memory=true: dial tcp: lookup workers-p90a on 10.245.0.10:53: no such host]
1 manager.go:111] unable to fully collect metrics: unable to fully scrape metrics from source kubelet_summary:samsungserver: unable to fetch metrics from Kubelet samsungserver (fd0e:72bc:2011::11): Get https://[fd0e:72bc:2011::11]:10250/stats/summary?only_cpu_and_memory=true: dial tcp [fd0e:72bc:2011::11]:10250: connect: network is unreachable

### Auth

E0129 00:52:48.373899       1 manager.go:102] unable to fully collect metrics: [unable to fully scrape metrics from source kubelet_summary:ip-10-132-9-84.us-west-2.compute.internal: unable to fetch metrics from Kubelet ip-10-132-9-84.us-west-2.compute.internal (10.132.9.84): request failed - "401 Unauthorized

Metrics server may fail to authenticate if kubelet is running with --anonymous-auth=false flag.
Passing --authentication-token-webhook=true and --authorization-mode=Webhook flags to kubelet can fix this.


response: "Forbidden (user=system:serviceaccount:kube-system:metrics-server, verb=get, resource=nodes, subresource=stats)

#### Incorrect Kubelet cert configuration

E0116 18:28:38.488216       1 manager.go:111] unable to fully collect metrics: [unable to fully scrape metrics from source kubelet_summary:k8s-node3: unable to fetch metrics from Kubelet k8s-node3 (192.168.0.33): Get https://192.168.0.33:10250/stats/summary?only_cpu_and_memory=true: x509: cannot validate certificate for 192.168.0.33 because it doesn't contain any IP SANs, unable to fully scrape metrics from source kubelet_summary:k8s-node1: unable to fetch metrics from Kubelet k8s-node1 (192.168.0.31): Get https://192.168.0.31:10250/stats/summary?only_cpu_and_memory=true: x509: cannot validate certificate for 192.168.0.31 because it doesn't contain any IP SANs, unable to fully scrape metrics from source kubelet_summary:k8s-node2: unable to fetch metrics from Kubelet k8s-node2 (192.168.0.32): Get https://192.168.0.32:10250/stats/summary?only_cpu_and_memory=true: x509: cannot validate certificate for 192.168.0.32 because it doesn't contain any IP SANs]
- --requestheader-client-ca-file=/front-proxy-ca.pem

#### Kubelet authorization

  kubelet:
    anonymousAuth: false
    authenticationTokenWebhook: true
    authorizationMode: Webhook

#### Apiserver cannot access metrics-server
I1217 11:12:04.139413       1 log.go:172] http: TLS handshake error from X.X.X.X:39968: EOF

https://kubernetes.io/docs/tasks/access-kubernetes-api/configure-aggregation-layer/
https://kubernetes.io/docs/concepts/architecture/master-node-communication/#master-to-cluster

#### Problem of certificate rotation on EKS

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