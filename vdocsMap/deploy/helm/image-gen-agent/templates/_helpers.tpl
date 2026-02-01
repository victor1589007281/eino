{{/*
展开 Chart 名称
*/}}
{{- define "image-gen-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
创建完整名称，截断到 63 个字符（K8s 名称限制）
*/}}
{{- define "image-gen-agent.fullname" -}}
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
创建 Chart 标签
*/}}
{{- define "image-gen-agent.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
通用标签
*/}}
{{- define "image-gen-agent.labels" -}}
helm.sh/chart: {{ include "image-gen-agent.chart" . }}
{{ include "image-gen-agent.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
选择器标签
*/}}
{{- define "image-gen-agent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "image-gen-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
服务账户名称
*/}}
{{- define "image-gen-agent.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "image-gen-agent.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
配置文件内容
*/}}
{{- define "image-gen-agent.configContent" -}}
server:
  host: {{ .Values.config.server.host | quote }}
  port: {{ .Values.config.server.port }}
  mode: {{ .Values.config.server.mode | quote }}

llm:
  provider: {{ .Values.config.llm.provider | quote }}
  model: {{ .Values.config.llm.model | quote }}
  max_tokens: {{ .Values.config.llm.maxTokens }}
  temperature: {{ .Values.config.llm.temperature }}
  timeout: {{ .Values.config.llm.timeout }}

image:
  static_image:
    provider: {{ .Values.config.image.staticImage.provider | quote }}
    model: {{ .Values.config.image.staticImage.model | quote }}
    default_size: {{ .Values.config.image.staticImage.defaultSize | quote }}
    quality: {{ .Values.config.image.staticImage.quality | quote }}
  animated_gif:
    provider: {{ .Values.config.image.animatedGif.provider | quote }}
    default_fps: {{ .Values.config.image.animatedGif.defaultFps }}
    max_duration: {{ .Values.config.image.animatedGif.maxDuration }}
    default_size: {{ .Values.config.image.animatedGif.defaultSize | quote }}
  sticker:
    provider: {{ .Values.config.image.sticker.provider | quote }}
    default_size: {{ .Values.config.image.sticker.defaultSize | quote }}
    with_text: {{ .Values.config.image.sticker.withText }}
    text_position: {{ .Values.config.image.sticker.textPosition | quote }}
  output:
    directory: {{ .Values.config.image.output.directory | quote }}
    max_file_size: {{ .Values.config.image.output.maxFileSize }}
    format: {{ .Values.config.image.output.format | quote }}

storage:
  type: "local"
  base_path: "/data"

agent:
  name: {{ .Values.config.agent.name | quote }}
  max_iterations: {{ .Values.config.agent.maxIterations }}
  retry_count: {{ .Values.config.agent.retryCount }}
  retry_delay: {{ .Values.config.agent.retryDelay }}
  enabled_tools:
  {{- range .Values.config.agent.enabledTools }}
    - {{ . | quote }}
  {{- end }}

logging:
  level: {{ .Values.config.logging.level | quote }}
  format: {{ .Values.config.logging.format | quote }}
  output_path: {{ .Values.config.logging.outputPath | quote }}
{{- end }}
