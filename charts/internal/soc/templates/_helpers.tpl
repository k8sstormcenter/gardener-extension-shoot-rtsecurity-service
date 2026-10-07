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

{{/*
Namespaces the sensor ignores entirely. The target's namespace is NOT one of them, even
before it has a profile: an ignored namespace produces no profiles either, and the evidence
from ungoverned runs is exactly what the signoff loop needs as input.
*/}}
{{- define "soc.kubescape.excludeNamespaces" -}}
{{- .Values.kubescape.excludeNamespaces | uniq | join "," -}}
{{- end }}
{{/*
Namespaces the alert binding leaves out: the same list the sensor ignores, plus the
target's while it has no signed-off profile.

The same list on purpose. A namespace the sensor skips produces no profiles at all, so
rules bound there can never fire whatever the binding says — the sensor's list always
wins. Keeping two lists is exactly how a cluster ends up with no workload profiles, no
alerts, and every component reporting healthy.

The target is the one legitimate difference: watched so its runs are learned, not judged
until there is a signed-off profile to judge it by.
*/}}
{{- define "soc.kubescape.bindingExcludeNamespaces" -}}
{{- $ns := .Values.kubescape.excludeNamespaces -}}
{{- if and .Values.target.enabled (not .Values.profiles.enabled) -}}
{{- $ns = append $ns .Values.target.namespace -}}
{{- end -}}
{{- toJson ($ns | uniq) -}}
{{- end }}
