# Known issues

## Table of Contents

<!-- toc -->
- [Kubelet doesn't report metrics for all or subset of nodes](#kubelet-doesnt-report-metrics-for-all-or-subset-of-nodes)
- [Kubelet doesn't report pod metrics](#kubelet-doesnt-report-pod-metrics)
- [Metrics-server pod failed to reach running status](#metrics-server-pod-failed-to-reach-running-status)
- [HPA is unable to get resource utilization](#hpa-is-unable-to-get-resource-utilization)
- [Incorrectly configured front-proxy certificate](#incorrectly-configured-front-proxy-certificate)
- [Network problem when connecting with Kubelet](#network-problem-when-connecting-with-kubelet)
- [Unable to work properly in Amazon EKS](#unable-to-work-properly-in-amazon-eks)
<!-- /toc -->

## Kubelet doesn't report metrics for all or subset of nodes

**Symptoms**

If you run `kubectl top nodes` and not get metrics for all nods in the clusters or some nodes will return `<unknown>`  value. For example:
```
$ kubectl top nodes
NAME      CPU(cores)  CPU%       MEMORY(bytes)  MEMORY%
master-1  192m        2%         10874Mi        68%
node-1    582m        7%         9792Mi         61%
node-2    <unknown>   <unknown>  <unknown>      <unknown>
```

**Debugging**

Please check if your Kubelet returns correct node metrics.

- Version 0.6.x and later

You can do that by checking resource metrics on nodes that are missing metrics.

```console
NODE_NAME=<Name of node in your cluster>
kubectl get --raw /api/v1/nodes/$NODE_NAME/proxy/metrics/resource
```

If usage values are equal zero or timestamp is lost, means that there is problem is related to Kubelet and not Metrics Server. Metrics Server require value and timestamp of  `node_cpu_usage_seconds_total`  `node_memory_working_set_bytes`  `container_cpu_usage_seconds_total` and `container_memory_working_set_bytes` to not be zero or missed.

- Version 0.5.x and earlier

You can do that by checking Summary API on nodes that are missing metrics.

You can read JSON response returned by command below and check `.node.cpu` and `.node.memory` fields. 
```console
NODE_NAME=<Name of node in your cluster>
kubectl get --raw /api/v1/nodes/$NODE_NAME/proxy/stats/summary
```

Alternativly you can run one liner using (requires [jq](https://stedolan.github.io/jq/)):
```
NODE_NAME=<Name of node in your cluster>
kubectl get --raw /api/v1/nodes/$NODE_NAME/proxy/stats/summary | jq '{cpu: .node.cpu, memory: .node.memory}'
```

If usage values are equal zero, means that there is problem is related to Kubelet and not Metrics Server. Metrics Server requires that `.cpu.usageCoreNanoSeconds` or `.memory.workingSetBytes` to not be zero. Example of invalid result:
```json
{
  "cpu": {
    "time": "2022-01-19T13:12:56Z",
    "usageNanoCores": 0,
    "usageCoreNanoSeconds": 0
  },
  "memory": {
    "time": "2022-01-19T13:12:56Z",
    "availableBytes": 16769261568,
    "usageBytes": 0,
    "workingSetBytes": 0,
    "rssBytes": 0,
    "pageFaults": 0,
    "majorPageFaults": 0
  }
}
```

**Known causes**

* Nodes use cgroupv2 that are not supported as of Kubernetes 1.21.

**Workaround**

* Reconfigure/rollback the nodes to use use cgroupv1. For non-production clusters you might want to alternatily try out cgroupv2 alpha support in Kubernetes v1.22 https://github.com/kubernetes/enhancements/issues/2254.

## Kubelet doesn't report pod metrics

If you run `kubectl top pods` and not get metrics for the pod, even though pod is already running for over 1 minute. To confirm please check that Metrics Server has logged that there were not metrics available for pod.

You can get Metrics Server logs by running `kubectl -n kube-system logs -l k8s-app=metrics-server` (works with official manifests, if you installed Metrics Server different way you might need to tweak the command). Example log line you are looking for:
```
W1106 20:50:15.238493 73592 top_pod.go:265] Metrics not available for pod default/example, age: 22h29m11.238478174s
error: Metrics not available for pod default/example, age: 22h29m11.238478174s
```

**Debugging**

Please check if your Kubelet is correctly returning pod metrics. 

- Version 0.6.x and later

You can do that by checking resource metrics on node where pod with missing metrics is running (can be checked by running `kubectl -n <pod_namespace> describe pod <pod_name>`:
```console
NODE_NAME=<Name of node in your cluster>
kubectl get --raw /api/v1/nodes/$NODE_NAME/proxy/metrics/resource
```

- Version 0.5.x and earlier

You can do that by checking Summary API on node where pod with missing metrics is running (can be checked by running `kubectl -n <pod_namespace> describe pod <pod_name>`:
```console
NODE_NAME=<Name of node where pod runs>
kubectl get --raw /api/v1/nodes/$NODE_NAME/proxy/stats/summary
```

This will return JSON that will have two keys `node` and `pods`. 
Empty list of pods means that problem is related to Kubelet and not to Metrics Server.

One liner for number of pod metrics reported by first node in cluster (requires [jq](https://stedolan.github.io/jq/)):
```console
kubectl get --raw /api/v1/nodes/$(kubectl get nodes -o json  | jq -r '.items[0].metadata.name')/proxy/stats/summary | jq '.pods | length'
```

**Known causes**

* [Kubelet doesn't report pod metrics in Kubernetes 1.19 with Docker <v19](https://github.com/kubernetes/kubernetes/issues/94281)
  
  **Workaround**
  
  Upgrade Docker to v19.03


* [Minikube doesn't enable Kubelet metrics by default](https://github.com/kubernetes-sigs/metrics-server/issues/1018)

  **Workaround**

  When launching minikube set `kubelet.housekeeping-interval`, for example `minikube start --extra-config=kubelet.housekeeping-interval=10s`

## Metrics-server pod failed to reach running status

**Symptoms**

When running `kubectl get pods -n kube-system` and not get running status for the metrics-server pod.

```
NAME                                           READY   STATUS    RESTARTS   AGE
metrics-server-dbf765b9b-mhqm7                 0/1     Running   0          11m
```

**Debugging**

Please check if your metrics server reports problems to scrape node, in particular errors will include
```
"Failed to scrape node" err="Get "https://192.168.65.4:10250/metrics/resource\": context deadline exceeded" node="docker-desktop"
"Failed probe" probe="metric-storage-ready" err="no metrics to serve"
```

**Known Causes**

metrics-server scrape kubelet `/metrics/resource` or `/stats/summary` endpoint timeout.

**Workaround**

* Please check the network status in the environment. Make sure that metrics-server can access kubelet's `/metrics/resource` (metrics-server v0.6.x and later) or `stats/summary` (metrics-server v0.5.x and earlier) endpoint normally.

* Some Docker Desktop on Apple M1 environments may take more than 30s to access the kubelet `/metrics/resource` endpoint. If this is the case, please report to the repo `docker/for-mac`.

## HPA is unable to get resource utilization

**Symptoms**


When running Deployment with horizontal autoscaling, HPA fails to compute replica count and reports error `failed to get memory utilization: missing request for memory`, even though Deployment sets resource requests. For example when running:
```
$ kubectl describe hpa 
```
will return
```
Name:                                                     pwa
Namespace:                                                pwa
Labels:                                                   app=pwa
env=prod
Annotations:                                              <none>
CreationTimestamp:                                        Wed, 23 Mar 2022 12:29:16 +0000
Reference:                                                Deployment/pwa
Metrics:                                                  ( current / target )
resource memory on pods  (as a percentage of request):  <unknown> / 80%
resource cpu on pods  (as a percentage of request):     <unknown> / 70%
Min replicas:                                             14
Max replicas:                                             24
Deployment pods:                                          16 current / 0 desired
Conditions:
Type           Status  Reason                   Message
  ----           ------  ------                   -------
AbleToScale    True    SucceededGetScale        the HPA controller was able to get the target's current scale
ScalingActive  False   FailedGetResourceMetric  the HPA was unable to compute the replica count: failed to get memory utilization: missing request for memory
Events:
Type     Reason                        Age                   From                       Message
  ----     ------                        ----                  ----                       -------
Warning  FailedGetResourceMetric       17m (x8 over 19m)     horizontal-pod-autoscaler  failed to get cpu utilization: missing request for cpu
Warning  FailedComputeMetricsReplicas  17m (x8 over 19m)     horizontal-pod-autoscaler  invalid metrics (2 invalid out of 2), first error is: failed to get memory utilization: missing request for memory
Warning  FailedGetResourceMetric       4m32s (x61 over 19m)  horizontal-pod-autoscaler  failed to get memory utilization: missing request for memory
```
**Debugging**

* Check if all pods under HPA label selector have cpu and memory requests set.

**Known solutions**

* Set cpu and memory requests on all pods under label selector.
* Change HPA label selector or pods labels so that pods without requests are no longer picked up.  

## Incorrectly configured front-proxy certificate

**Symptoms**


**Debuging**

Please check if your metrics server reports problem with authenticating client certificate, in particular errors mentioning `Unable to authenticate the request due to an error`. You can check logs by running command:
```
kubectl logs -n kube-system -l k8s-app=metrics-server --container metrics-server
```

Problem with front-proxy certificate can be recognized if logs have line similar to one below:
```
E0524 01:37:36.055326       1 authentication.go:65] Unable to authenticate the request due to an error: x509: certificate signed by unknown authority
```

**Known Causes**

* kubeadm uses separate `front-proxy` certificates that are not signed by main cluster certificate authority. 

  To fix this problem you need to provide kube-apiserver proxy-client CA to Metrics Server under `--requestheader-client-ca-file` flag. You can read more about this flag in [Authenticating Proxy](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#authenticating-proxy)




  1. Find your front-proxy certificates by checking arguments passed in kube-apiserver config (by default located in /etc/kubernetes/manifests/kube-apiserver.yaml)
  
    ```
    - --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt
    - --proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client.key
    ```

  2. Create configmap including `front-proxy-ca.crt`

    ```
    kubectl -nkube-system create configmap front-proxy-ca --from-file=front-proxy-ca.crt=/etc/kubernetes/pki/front-proxy-ca.crt -o yaml | kubectl -nkube-system replace configmap front-proxy-ca -f -
    ```

  3. Mount configmap in Metrics Server deployment and add `--requestheader-client-ca-file` flag

  ```
        - args:
          - --requestheader-client-ca-file=/ca/front-proxy-ca.crt // ADD THIS!
          - --cert-dir=/tmp
          - --secure-port=10250
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

## Network problem when connecting with Kubelet

**Symptoms**

When running `kubectl top nodes` we get partial or no information. For example results like:
```
NAME         CPU(cores) CPU%      MEMORY(bytes)   MEMORY%     
k8s-node01   59m        5%        1023Mi          26%         
k8s-master   <unknown>  <unknown> <unknown>       <unknown>               
k8s-node02   <unknown>  <unknown> <unknown>       <unknown>         
```

**Debugging**

Please check if your metrics server reports problems with connecting to Kubelet address, in particular errors will include `dial tcp IP(or hostname):10250: i/o timeout`. You can check logs by running command:

```
kubectl logs -n kube-system -l k8s-app=metrics-server --container metrics-server
```

Problem with network can be recognized if logs have line similar to one below:
```
unable to fully collect metrics: [unable to fully scrape metrics from source kubelet_summary:k8s-master: unable to fetch metrics from Kubelet k8s-master
(192.168.17.150): Get https://192.168.17.150:10250/stats/summary?only_cpu_and_memory=true: dial tcp 192.168.17.150:10250: i/o timeout
```

**Known solutions**
* [Calico] Check whether the value of `CALICO_IPV4POOL_CIDR` in the calico.yaml conflicts with the local physical network segment. The default: `192.168.0.0/16`.

  See [Kubernetes in Calico] for more details.

[Kubernetes in Calico]: https://docs.projectcalico.org/getting-started/kubernetes/quickstart

* [Firewall rules/Security groups] Check firewall configuration on your nodes. The reason may be in closed ports for incoming connections, so the metrics server cannot collect metrics from other nodes.

## Unable to work properly in Amazon EKS

**Symptoms**

When running metrics-server in Amazon EKS, can't collect metrics from containers, pods, or nodes.

**Debugging**

Please check the metrics-server log, whether there is the following information

```
E0413 12:28:25.449973 1 authentication.go:65] Unable to authenticate the request due to an error: x509: certificate signed by unknown authority
```

**Known solutions**

* If you access your cluster through a role defined in the aws-auth ConfigMap, confirm that you have set the username field and the mapping.

1. To describe the aws-auth ConfigMap, run the following command:
```
$ kubectl describe -n kube-system configmap aws-auth
```
2. Confirm that the username field is set for the role accessing the cluster. See the following example:
```
Name:         aws-auth
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>

Data

mapRoles:

-
groups:
- system:masters
  rolearn: arn:aws:iam::123456789123:role/kubernetes-devops
  username: devops:{{SessionName}}  # Ensure this has been specified.
```
* See [Why can't I collect metrics from containers, pods, or nodes using Metrics Server in Amazon EKS?] for more details.

[Why can't I collect metrics from containers, pods, or nodes using Metrics Server in Amazon EKS?]: https://aws.amazon.com/premiumsupport/knowledge-center/eks-metrics-server/

