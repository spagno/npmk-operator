{{/*
Expand the name of the chart.
*/}}
{{- define "npmk-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "npmk-operator.fullname" -}}
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
{{- define "npmk-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "npmk-operator.labels" -}}
helm.sh/chart: {{ include "npmk-operator.chart" . }}
{{ include "npmk-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "npmk-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "npmk-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: npmk-operator
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "npmk-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "npmk-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
NPMKO credentials secret name
*/}}
{{- define "npmk-operator.credentialsSecretName" -}}
{{- if .Values.npm.existingSecret }}
{{- .Values.npm.existingSecret }}
{{- else }}
{{- printf "%s-credentials" (include "npmk-operator.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Config map name
*/}}
{{- define "npmk-operator.configMapName" -}}
{{- printf "%s-config" (include "npmk-operator.fullname" .) }}
{{- end }}
