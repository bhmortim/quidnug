{{/*
Expand the name of the chart.
*/}}
{{- define "quidnug.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "quidnug.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Chart name + version as used in labels.
*/}}
{{- define "quidnug.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels applied to every resource.
*/}}
{{- define "quidnug.labels" -}}
helm.sh/chart: {{ include "quidnug.chart" . }}
{{ include "quidnug.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels — stable subset of common labels.
*/}}
{{- define "quidnug.selectorLabels" -}}
app.kubernetes.io/name: {{ include "quidnug.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
ServiceAccount name.
*/}}
{{- define "quidnug.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "quidnug.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Quid secret name. Either the user-provided existingQuidSecret or a chart-owned one.
*/}}
{{- define "quidnug.quidSecretName" -}}
{{- if .Values.existingQuidSecret -}}
{{- .Values.existingQuidSecret -}}
{{- else -}}
{{- printf "%s-quid" (include "quidnug.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Image reference.
*/}}
{{- define "quidnug.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}
