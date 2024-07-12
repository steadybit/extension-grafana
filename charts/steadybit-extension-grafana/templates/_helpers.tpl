{{/* vim: set filetype=mustache: */}}

{{/*
Expand the name of the chart.
*/}}
{{- define "grafana.secret.name" -}}
{{- default "steadybit-extension-grafana" .Values.grafana.existingSecret -}}
{{- end -}}
