{{/*
Expand the name of the chart.
*/}}
{{- define "image-optimize-proxy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "image-optimize-proxy.fullname" -}}
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
Create chart label.
*/}}
{{- define "image-optimize-proxy.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "image-optimize-proxy.labels" -}}
helm.sh/chart: {{ include "image-optimize-proxy.chart" . }}
{{ include "image-optimize-proxy.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "image-optimize-proxy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "image-optimize-proxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "image-optimize-proxy.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "image-optimize-proxy.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Affinity: merges HA podAntiAffinity (when ha.enabled) with user-supplied .Values.affinity.
Returns empty dict when neither ha.enabled nor .Values.affinity is set.
*/}}
{{- define "image-optimize-proxy.affinity" -}}
{{- $default := dict }}
{{- if .Values.ha.enabled }}
  {{- $matchLabels := dict "app.kubernetes.io/name" (include "image-optimize-proxy.name" .) }}
  {{- $topologyKey := "kubernetes.io/hostname" }}
  {{- $antiAffinity := dict }}
  {{- if .Values.ha.podAntiAffinity.required }}
    {{- $_ := set $antiAffinity "requiredDuringSchedulingIgnoredDuringExecution" (list (dict
        "labelSelector" (dict "matchLabels" $matchLabels)
        "topologyKey" $topologyKey
    )) }}
  {{- else }}
    {{- $_ := set $antiAffinity "preferredDuringSchedulingIgnoredDuringExecution" (list (dict
        "weight" 100
        "podAffinityTerm" (dict
          "labelSelector" (dict "matchLabels" $matchLabels)
          "topologyKey" $topologyKey
        )
    )) }}
  {{- end }}
  {{- $_ := set $default "podAntiAffinity" $antiAffinity }}
{{- end }}
{{- toYaml (merge $default .Values.affinity) }}
{{- end }}

{{/*
topologySpreadConstraints: zone-level spread (maxSkew 1, ScheduleAnyway).
Only rendered when .Values.ha.enabled is true.
*/}}
{{- define "image-optimize-proxy.topologySpreadConstraints" -}}
- maxSkew: 1
  topologyKey: topology.kubernetes.io/zone
  whenUnsatisfiable: ScheduleAnyway
  labelSelector:
    matchLabels:
      {{- include "image-optimize-proxy.selectorLabels" . | nindent 6 }}
{{- end }}
