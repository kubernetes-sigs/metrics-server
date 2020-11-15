---
name: Bug Report
about: Report a bug encountered while operating Metrics Server

---

<!-- Please use this template while reporting a bug and provide as much info as possible. Not doing so may result in your bug not being addressed in a timely manner. Thanks!-->


**What happened**:

**What you expected to happen**:

**Anything else we need to know?**:

**Environment**:
- Kubernetes distribution (GKE, EKS, Kubeadm, the hard way, etc.):
- Container Network Setup (flannel, calico, etc.):
- Kubernetes version (use `kubectl version`):

- Metrics Server manifest

<details>
  <summary>spoiler for Metrics Server manifest:</summary>

  <!--- INSERT manifest HERE --->

</details>

- Kubelet config:

<details>
  <summary>spoiler for Kubelet config:</summary>

  <!--- INSERT kubelet config HERE --->

</details>

- Metrics server logs:

<details>
  <summary>spoiler for Metrics Server logs:</summary>

  <!--- INSERT logs HERE --->

</details>

- Status of Metrics API:
<details>
  <summary>spolier for Status of Metrics API:</summary>

  ```
  kubectl describe apiservice v1beta1.metrics.k8s.io
  ```

  <!--- INSERT results of command above --->

</details>

<!-- DO NOT EDIT BELOW THIS LINE -->
/kind bug
