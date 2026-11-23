{{/*
Expand the name of the chart.
*/}}
{{- define "metrics-server.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "metrics-server.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Allow the release namespace to be overridden for multi-namespace deployments in combined charts
*/}}
{{- define "metrics-server.namespace" -}}
{{- .Values.namespaceOverride | default .Release.Namespace -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "metrics-server.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "metrics-server.labels" -}}
helm.sh/chart: {{ include "metrics-server.chart" . }}
{{ include "metrics-server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Values.commonLabels }}
{{ toYaml .Values.commonLabels }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "metrics-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "metrics-server.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "metrics-server.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "metrics-server.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
The image to use
*/}}
{{- define "metrics-server.image" -}}
{{- printf "%s:%s" .Values.image.repository (default (printf "v%s" .Chart.AppVersion) .Values.image.tag) }}
{{- end }}

{{/*
The image to use for the addon resizer
*/}}
{{- define "metrics-server.addonResizer.image" -}}
{{- printf "%s:%s" .Values.addonResizer.image.repository .Values.addonResizer.image.tag }}
{{- end }}

{{/*
ConfigMap name of addon resizer
*/}}
{{- define "metrics-server.addonResizer.configMap" -}}
{{- printf "%s-%s" (include "metrics-server.fullname" .) "nanny-config" }}
{{- end }}

{{/*
Role name of addon resizer
*/}}
{{- define "metrics-server.addonResizer.role" -}}
{{ printf "system:%s-nanny" (include "metrics-server.fullname" .) }}
{{- end }}

{{/* Get PodDisruptionBudget API Version */}}
{{- define "metrics-server.pdb.apiVersion" -}}
  {{- if and (.Capabilities.APIVersions.Has "policy/v1") (semverCompare ">= 1.21-0" .Capabilities.KubeVersion.Version) -}}
      {{- print "policy/v1" -}}
  {{- else -}}
    {{- print "policy/v1beta1" -}}
  {{- end -}}
{{- end -}}

{{/*
APIService TLS verification fields.
Kubernetes rejects insecureSkipTLSVerify: true when caBundle is set, including when
cert-manager later injects caBundle via cert-manager.io/inject-ca-from.
Do not render insecureSkipTLSVerify true in those cases. An explicit false is still rendered.
Default tls.type metrics-server keeps insecureSkipTLSVerify true and no caBundle.
Context dict keys: root, certs, previous, existing.
*/}}
{{- define "metrics-server.apiServiceTLS" -}}
{{- $root := .root -}}
{{- $certs := .certs -}}
{{- $previous := .previous -}}
{{- $existing := .existing -}}
{{- $hasCABundle := false -}}
{{- $caBundle := "" -}}
{{- if eq $root.Values.tls.type "helm" -}}
  {{- $hasCABundle = true -}}
  {{- if and $previous $root.Values.tls.helm.lookup -}}
    {{- $caBundle = index $previous.data "tls.crt" -}}
  {{- else -}}
    {{- $caBundle = $certs.Cert | b64enc -}}
  {{- end -}}
{{- else if and $existing $existing.data -}}
  {{- $hasCABundle = true -}}
  {{- $caBundle = index $existing.data "tls.crt" -}}
{{- else if and $root.Values.apiService.caBundle (ne $root.Values.tls.type "cert-manager") -}}
  {{- $hasCABundle = true -}}
  {{- $caBundle = $root.Values.apiService.caBundle | b64enc -}}
{{- end -}}
{{- $skipTLSVerify := $root.Values.apiService.insecureSkipTLSVerify -}}
{{- $certManagerInjectsCA := and $root.Values.tls.certManager.addInjectorAnnotations (eq $root.Values.tls.type "cert-manager") -}}
{{- $renderSkipTLSVerify := false -}}
{{- if and (not $hasCABundle) (not $certManagerInjectsCA) -}}
  {{- if ne ($skipTLSVerify | toString) "<nil>" -}}
    {{- $renderSkipTLSVerify = true -}}
  {{- end -}}
{{- else if not $skipTLSVerify -}}
  {{- $renderSkipTLSVerify = true -}}
  {{- $skipTLSVerify = false -}}
{{- end -}}
{{- if $hasCABundle }}
  caBundle: {{ $caBundle }}
{{- end }}
{{- if $renderSkipTLSVerify }}
  insecureSkipTLSVerify: {{ $skipTLSVerify }}
{{- end }}
{{- end -}}
