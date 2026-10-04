{{- define "soc.clickhouse.httpEndpoint" -}}
{{- if eq .Values.clickhouse.mode "central" -}}
{{ .Values.clickhouse.central.httpEndpoint }}
{{- else -}}
http://clickhouse-forensic-soc-db.{{ .Values.namespaces.clickhouse }}.svc.cluster.local:8123
{{- end -}}
{{- end -}}
