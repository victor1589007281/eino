{{/*
Expand the name of the chart.
*/}}
{{- define "kernel-expert.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kernel-expert.fullname" -}}
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
{{- define "kernel-expert.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kernel-expert.labels" -}}
helm.sh/chart: {{ include "kernel-expert.chart" . }}
{{ include "kernel-expert.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kernel-expert.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kernel-expert.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kernel-expert.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kernel-expert.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Redis URL
*/}}
{{- define "kernel-expert.redisUrl" -}}
{{- if .Values.redis.enabled }}
{{- printf "redis://:%s@%s-redis-master:6379" .Values.redis.auth.password .Release.Name }}
{{- else }}
{{- .Values.secrets.redisUrl }}
{{- end }}
{{- end }}

{{/*
PostgreSQL URL
*/}}
{{- define "kernel-expert.postgresUrl" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "postgres://postgres:%s@%s-postgresql:5432/%s?sslmode=disable" .Values.postgresql.auth.postgresPassword .Release.Name .Values.postgresql.auth.database }}
{{- else }}
{{- .Values.secrets.postgresUrl }}
{{- end }}
{{- end }}
