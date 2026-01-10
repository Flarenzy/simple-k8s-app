{{/*
Expand the name of the chart.
*/}}
{{- define "ipam.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ipam.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "ipam.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ipam.labels" -}}
helm.sh/chart: {{ include "ipam.chart" . }}
{{ include "ipam.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "ipam.apiLabels" -}}
{{ include "ipam.labels" . }}
app.kubernetes.io/component: api
{{- end }}

{{- define "ipam.feLabels" -}}
{{ include "ipam.labels" . }}
app.kubernetes.io/component: fe
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ipam.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ipam.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "ipam.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ipam.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "ipam.apiFullname" -}}{{ include "ipam.fullname" . }}-api{{- end -}}
{{- define "ipam.feFullname" -}}{{ include "ipam.fullname" . }}-fe{{- end -}}
{{- define "ipam.keycloakFullname" -}}{{ include "ipam.fullname" . }}-keycloak{{- end -}}

{{- define "ipam.dbSecretName" -}}
{{- if .Values.db.existingSecret }}
{{- .Values.db.existingSecret }}
{{- else }}
{{- printf "%s-db" (include "ipam.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "ipam.apiSelectorLabels" -}}
{{ include "ipam.selectorLabels" . }}
app.kubernetes.io/component: api
{{- end -}}

{{- define "ipam.feSelectorLabels" -}}
{{ include "ipam.selectorLabels" . }}
app.kubernetes.io/component: fe
{{- end -}}

{{- define "ipam.keycloakLabels" -}}
{{ include "ipam.labels" . }}
app.kubernetes.io/component: keycloak
{{- end -}}

{{- define "ipam.keycloakSelectorLabels" -}}
{{ include "ipam.selectorLabels" . }}
app.kubernetes.io/component: keycloak
{{- end -}}
