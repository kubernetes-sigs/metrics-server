# Kubernetes Metrics Server

[Metrics Server](https://github.com/kubernetes-sigs/metrics-server/) is a scalable, efficient source of container resource metrics for Kubernetes built-in autoscaling pipelines.

## Installing the Chart

Before you can install the chart you will need to add the `metrics-server` repo to [Helm](https://helm.sh/).

```shell
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
```

After you've installed the repo you can install the chart.

```shell
helm upgrade --install metrics-server metrics-server/metrics-server
```

## Configuration

The following table lists the configurable parameters of the _Metrics Server_ chart and their default values.

| Parameter                                        | Description                                                                                                                                                                                                                                                      | Default                                                                        |
| ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| `image.repository`                               | Image repository.                                                                                                                                                                                                                                                | `registry.k8s.io/metrics-server/metrics-server`                                |
| `image.tag`                                      | Image tag, will override the default tag derived from the chart app version.                                                                                                                                                                                     | `""`                                                                           |
| `image.pullPolicy`                               | Image pull policy.                                                                                                                                                                                                                                               | `IfNotPresent`                                                                 |
| `imagePullSecrets`                               | Image pull secrets.                                                                                                                                                                                                                                              | `[]`                                                                           |
| `nameOverride`                                   | Override the `name` of the chart.                                                                                                                                                                                                                                | `nil`                                                                          |
| `fullnameOverride`                               | Override the `fullname` of the chart.                                                                                                                                                                                                                            | `nil`                                                                          |
| `serviceAccount.create`                          | If `true`, create a new service account.                                                                                                                                                                                                                         | `true`                                                                         |
| `serviceAccount.annotations`                     | Annotations to add to the service account.                                                                                                                                                                                                                       | `{}`                                                                           |
| `serviceAccount.name`                            | Service account to be used. If not set and `serviceAccount.create` is `true`, a name is generated using the full name template.                                                                                                                                  | `nil`                                                                          |
| `serviceAccount.secrets`                         | The list of secrets mountable by this service account. See <https://kubernetes.io/docs/reference/labels-annotations-taints/#enforce-mountable-secrets>                                                                                                           | `[]`                                                                           |
| `rbac.create`                                    | If `true`, create the RBAC resources.                                                                                                                                                                                                                            | `true`                                                                         |
| `rbac.pspEnabled`                                | If `true`, create a pod security policy resource, unless Kubernetes version is 1.25 or later.                                                                                                                                                                    | `false`                                                                        |
| `apiService.create`                              | If `true`, create the `v1beta1.metrics.k8s.io` API service. You typically want this enabled! If you disable API service creation you have to manage it outside of this chart for e.g horizontal pod autoscaling to work with this release.                       | `true`                                                                         |
| `apiService.annotations`                         | Annotations to add to the API service                                                                                                                                                                                                                            | `{}`                                                                           |
| `apiService.insecureSkipTLSVerify`               | Specifies whether to skip TLS verification (NOTE: this setting is not a proxy for the `--kubelet-insecure-tls` metrics-server flag)                                                                                                                              | `true`                                                                         |
| `apiService.caBundle`                            | The PEM encoded CA bundle for TLS verification                                                                                                                                                                                                                   | `""`                                                                           |
| `commonLabels`                                   | Labels to add to each object of the chart.                                                                                                                                                                                                                       | `{}`                                                                           |
| `podLabels`                                      | Labels to add to the pod.                                                                                                                                                                                                                                        | `{}`                                                                           |
| `podAnnotations`                                 | Annotations to add to the pod.                                                                                                                                                                                                                                   | `{}`                                                                           |
| `podSecurityContext`                             | Security context for the pod.                                                                                                                                                                                                                                    | `{}`                                                                           |
| `securityContext`                                | Security context for the _metrics-server_ container.                                                                                                                                                                                                             | _See values.yaml_                                                              |
| `priorityClassName`                              | Priority class name to use.                                                                                                                                                                                                                                      | `system-cluster-critical`                                                      |
| `containerPort`                                  | port for the _metrics-server_ container.                                                                                                                                                                                                                         | `10250`                                                                        |
| `hostNetwork.enabled`                            | If `true`, start _metric-server_ in hostNetwork mode. You would require this enabled if you use alternate overlay networking for pods and API server unable to communicate with metrics-server. As an example, this is required if you use Weave network on EKS. | `false`                                                                        |
| `replicas`                                       | Number of replicas to run.                                                                                                                                                                                                                                       | `1`                                                                            |
| `revisionHistoryLimit`                           | Number of revisions to keep.                                                                                                                                                                                                                                     | `nil`                                                                          |
| `updateStrategy`                                 | Customise the default update strategy.                                                                                                                                                                                                                           | `{}`                                                                           |
| `podDisruptionBudget.enabled`                    | If `true`, create `PodDisruptionBudget` resource.                                                                                                                                                                                                                | `{}`                                                                           |
| `podDisruptionBudget.minAvailable`               | Set the `PodDisruptionBudget` minimum available pods.                                                                                                                                                                                                            | `nil`                                                                          |
| `podDisruptionBudget.maxUnavailable`             | Set the `PodDisruptionBudget` maximum unavailable pods.                                                                                                                                                                                                          | `nil`                                                                          |
| `podDisruptionBudget.maxUnavailable`             | Set the `PodDisruptionBudget` maximum unavailable pods.                                                                                                                                                                                                          | `nil`                                                                          |
| `podDisruptionBudget.unhealthyPodEvictionPolicy` | Unhealthy pod eviction policy for the PDB.                                                                                                                                                                                                                       | `nil`                                                                          |
| `defaultArgs`                                    | Default arguments to pass to the _metrics-server_ command.                                                                                                                                                                                                       | See _values.yaml_                                                              |
| `args`                                           | Additional arguments to pass to the _metrics-server_ command.                                                                                                                                                                                                    | `[]`                                                                           |
| `livenessProbe`                                  | Liveness probe.                                                                                                                                                                                                                                                  | See _values.yaml_                                                              |
| `readinessProbe`                                 | Readiness probe.                                                                                                                                                                                                                                                 | See _values.yaml_                                                              |
| `service.type`                                   | Service type.                                                                                                                                                                                                                                                    | `ClusterIP`                                                                    |
| `service.port`                                   | Service port.                                                                                                                                                                                                                                                    | `443`                                                                          |
| `service.annotations`                            | Annotations to add to the service.                                                                                                                                                                                                                               | `{}`                                                                           |
| `service.labels`                                 | Labels to add to the service.                                                                                                                                                                                                                                    | `{}`                                                                           |
| `addonResizer.enabled`                           | If `true`, run the addon-resizer as a sidecar to automatically scale resource requests with cluster size.                                                                                                                                                        | `false`                                                                        |
| `addonResizer.securityContext`                   | Security context for the _metrics_server_container.                                                                                                                                                                                                              | _See values.yaml                                                               |
| `addonResizer.image.repository`                  | addon-resizer image repository                                                                                                                                                                                                                                   | `registry.k8s.io/autoscaling/addon-resizer`                                    |
| `addonResizer.image.tag`                         | addon-resizer image tag                                                                                                                                                                                                                                          | `1.8.23`                                                                       |
| `addonResizer.resources`                         | Resource requests and limits for the _nanny_ container.                                                                                                                                                                                                          | `{ requests: { cpu: 40m, memory: 25Mi }, limits: { cpu: 40m, memory: 25Mi } }` |
| `addonResizer.nanny.cpu`                         | The base CPU requirement.                                                                                                                                                                                                                                        | `0m`                                                                           |
| `addonResizer.nanny.extraCPU`                    | The amount of CPU to add per node.                                                                                                                                                                                                                               | `1m`                                                                           |
| `addonResizer.nanny.memory`                      | The base memory requirement.                                                                                                                                                                                                                                     | `0Mi`                                                                          |
| `addonResizer.nanny.extraMemory`                 | The amount of memory to add per node.                                                                                                                                                                                                                            | `2Mi`                                                                          |
| `addonResizer.nanny.minClusterSize`              | Specifies the smallest number of nodes resources will be scaled to.                                                                                                                                                                                              | `100`                                                                          |
| `addonResizer.nanny.pollPeriod`                  | The time, in milliseconds, to poll the dependent container.                                                                                                                                                                                                      | `300000`                                                                       |
| `addonResizer.nanny.threshold`                   | A number between 0-100. The dependent's resources are rewritten when they deviate from expected by more than threshold.                                                                                                                                          | `5`                                                                            |
| `metrics.enabled`                                | If `true`, allow unauthenticated access to `/metrics`.                                                                                                                                                                                                           | `false`                                                                        |
| `serviceMonitor.enabled`                         | If `true`, create a _Prometheus_ service monitor. This needs `metrics.enabled` to be `true`.                                                                                                                                                                     | `false`                                                                        |
| `serviceMonitor.additionalLabels`                | Additional labels to be set on the ServiceMonitor.                                                                                                                                                                                                               | `{}`                                                                           |
| `serviceMonitor.metricRelabelings`               | _Prometheus_ metric relabeling.                                                                                                                                                                                                                                  | `[]`                                                                           |
| `serviceMonitor.relabelings`                     | _Prometheus_ relabeling.                                                                                                                                                                                                                                         | `[]`                                                                           |
| `serviceMonitor.interval`                        | _Prometheus_ scrape frequency.                                                                                                                                                                                                                                   | `1m`                                                                           |
| `serviceMonitor.scrapeTimeout`                   | _Prometheus_ scrape timeout.                                                                                                                                                                                                                                     | `10s`                                                                          |
| `resources`                                      | Resource requests and limits for the _metrics-server_ container. See <https://github.com/kubernetes-sigs/metrics-server#scaling>                                                                                                                                 | `{ requests: { cpu: 100m, memory: 200Mi }}`                                    |
| `extraVolumeMounts`                              | Additional volume mounts for the _metrics-server_ container.                                                                                                                                                                                                     | `[]`                                                                           |
| `extraVolumes`                                   | Additional volumes for the pod.                                                                                                                                                                                                                                  | `[]`                                                                           |
| `nodeSelector`                                   | Node labels for pod assignment.                                                                                                                                                                                                                                  | `{}`                                                                           |
| `tolerations`                                    | Tolerations for pod assignment.                                                                                                                                                                                                                                  | `[]`                                                                           |
| `affinity`                                       | Affinity for pod assignment.                                                                                                                                                                                                                                     | `{}`                                                                           |
| `topologySpreadConstraints`                      | Pod Topology Spread Constraints.                                                                                                                                                                                                                                 | `[]`                                                                           |
| `deploymentAnnotations`                          | Annotations to add to the deployment.                                                                                                                                                                                                                            | `{}`                                                                           |
| `schedulerName`                                  | scheduler to set to the deployment.                                                                                                                                                                                                                              | `""`                                                                           |
| `dnsConfig`                                      | Set the dns configuration options for the deployment.                                                                                                                                                                                                            | `{}`                                                                           |
| `tmpVolume`                                      | Volume to be mounted in Pods for temporary files.                                                                                                                                                                                                                | `{"emptyDir":{}}`                                                              |
| `tls.type`                                       | TLS option to use. Either use `metrics-server` for self-signed certificates, `helm`, `cert-manager` or `existingSecret`.                                                                                                                                         | `"metrics-server"`                                                             |
| `tls.clusterDomain`                              | Kubernetes cluster domain. Used to configure Subject Alt Names for the certificate when using `tls.type` `helm` or `cert-manager`.                                                                                                                               | `"cluster.local"`                                                              |
| `tls.certManager.addInjectorAnnotations`         | Automatically add the cert-manager.io/inject-ca-from annotation to the APIService resource.                                                                                                                                                                      | `true`                                                                         |
| `tls.certManager.existingIssuer.enabled`         | Use an existing cert-manager issuer                                                                                                                                                                                                                              | `false`                                                                        |
| `tls.certManager.existingIssuer.kind`            | Kind of the existing cert-manager issuer                                                                                                                                                                                                                         | `"Issuer"`                                                                     |
| `tls.certManager.existingIssuer.name`            | Name of the existing cert-manager issuer                                                                                                                                                                                                                         | `"my-issuer"`                                                                  |
| `tls.certManager.duration`                       | Set the requested duration (i.e. lifetime) of the Certificate.                                                                                                                                                                                                   | `""`                                                                           |
| `tls.certManager.renewBefore`                    | How long before the currently issued certificate’s expiry cert-manager should renew the certificate.                                                                                                                                                             | `""`                                                                           |
| `tls.certManager.annotations`                    | Add extra annotations to the Certificate resource                                                                                                                                                                                                                | `{}`                                                                           |
| `tls.certManager.labels`                         | Add extra labels to the Certificate resource                                                                                                                                                                                                                     | `{}`                                                                           |
| `tls.helm.certDurationDays`                      | Cert validity duration in days                                                                                                                                                                                                                                   | `365`                                                                          |
| `tls.helm.lookup`                                | Use helm lookup function to reuse Secret created in previous helm install                                                                                                                                                                                        | `true`                                                                         |
| `tls.existingSecret.lookup`                      | Use helm lookup function to provision `apiService.caBundle`                                                                                                                                                                                                      | `true`                                                                         |
| `tls.existingSecret.name`                        | Name of the existing Secret to use for TLS                                                                                                                                                                                                                       | `""`                                                                           |

## Hardening metrics-server

By default, metrics-server is using a self-signed certificate which is generated during startup. The `APIservice` resource is registered with `.spec.insecureSkipTLSVerify` set to `true` as you can see here:

```yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta1.metrics.k8s.io
spec:
  #..
  insecureSkipTLSVerify: true # <-- see here
  service:
    name: metrics-server
  #..
```

To harden metrics-server, you have these options described in the following section.

### Option 1: Let helm generate a self-signed certificate

This option is probably the easiest solution for you. We delegate the process to generate a self-signed certificate to helm.
As helm generates them during deploy time, helm can also inject the `apiService.caBundle` for you.

**The only disadvantage of using this method is that it is not GitOps friendly** (e.g. Argo CD). If you are using one of these
GitOps tools with drift detection, it will always detect changes. However if you are deploying the helm chart via Terraform
for example (or maybe even Flux), this method is perfectly fine.

To use this method, please setup your values file like this:

```yaml
apiService:
  insecureSkipTLSVerify: false
tls:
  type: helm
```

### Option 2: Use cert-manager

> **Requirement:** cert-manager needs to be installed before you install metrics-server

To use this method, please setup your values file like this:

```yaml
apiService:
  insecureSkipTLSVerify: false
tls:
  type: cert-manager
```

There are other optional parameters, if you want to customize the behavior of the certificate even more.

### Option 3: Use existing Secret

This option allows you to reuse an existing Secret. This Secrets can have an arbitrary origin, e.g.

- Created via kubectl / Terraform / etc.
- Synced from a secret management solution like AWS Secrets Manager, HashiCorp Vault, etc.

When using this type of TLS option, the keys `tls.key` and the `tls.crt` key must be provided in the data field of the
existing Secret.

You need to pass the certificate of the issuing CA (or the certificate itself) via `apiService.caBundle` to ensure
proper configuration of the `APIservice` resource. Otherwise you cannot set `apiService.insecureSkipTLSVerify` to
`false`.

To use this method, please setup your values file like this:

```yaml
apiService:
  insecureSkipTLSVerify: false
  caBundle: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----

tls:
  type: existingSecret
  existingSecret:
    name: metrics-server-existing
```
