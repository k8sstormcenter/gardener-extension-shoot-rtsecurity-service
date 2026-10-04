{{- define "soc.clickhouse.httpEndpoint" -}}
http://{{ include "soc.clickhouse.host" . }}:8123
{{- end -}}

{{- define "soc.clickhouse.host" -}}
{{- if eq .Values.clickhouse.mode "central" -}}{{ .Values.clickhouse.central.host }}{{- else -}}clickhouse-forensic-soc-db.{{ .Values.namespaces.clickhouse }}.svc.cluster.local{{- end -}}
{{- end }}
{{- define "soc.clickhouse.nativeDSN" -}}
{{ .Values.clickhouse.ingest.user }}:{{ .Values.clickhouse.ingest.password }}@{{ include "soc.clickhouse.host" . }}:9000/forensic_db
{{- end }}
{{- define "soc.clickhouse.httpURL" -}}
http://{{ .Values.clickhouse.ingest.user }}:{{ .Values.clickhouse.ingest.password }}@{{ include "soc.clickhouse.host" . }}:8123/forensic_db
{{- end }}
