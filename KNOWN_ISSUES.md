# Known issues

## Table of Contents

<!-- toc -->
- [Missing metrics](#missing-metrics)
- [Incorrectly configured front-proxy certificate](#incorrectly-configured-front-proxy-certificate)
- [Network problem when connecting with Kubelet](#network-problem-when-connecting-with-kubelet)
<!-- /toc -->

## Missing metrics 

**Symptoms**

Running
```
kubectl top pods
```
will return empty result for pods older then 1 minute. e.g.
```
W1106 20:50:15.238493 73592 top_pod.go:265] Metrics not available for pod default/example, age: 22h29m11.238478174s
error: Metrics not available for pod default/example, age: 22h29m11.238478174s
```

**Debugging**

Please check if your Kubelet is correctly returning pod metrics. You can do that by checking Summary API on any node in your cluster:
```console
NODE_NAME=<Name of node in your cluster>
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
          - --secure-port=4443
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
