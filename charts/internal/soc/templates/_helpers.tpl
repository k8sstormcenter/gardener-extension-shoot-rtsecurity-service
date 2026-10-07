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

{{- define "soc.vector.config" -}}
api:
  address: 127.0.0.1:8686
  enabled: true
data_dir: /vector-data-dir
sinks:
  dx_daemon:
    batch:
      max_events: 1
    buffer:
      max_size: 1073741824
      type: disk
      when_full: drop_newest
    encoding:
      codec: json
    inputs:
    - kubescape_enrich
    method: post
    tls:
      verify_certificate: false
    type: http
    uri: https://dx-daemon.{{ .Values.namespaces.honey }}.svc.cluster.local:9099/findings
  kubescape_clickhouse:
    auth:
      password: {{ .Values.clickhouse.ingest.password }}
      strategy: basic
      user: {{ .Values.clickhouse.ingest.user }}
    batch:
      max_bytes: 5000000
      timeout_secs: 2
    database: forensic_db
    date_time_best_effort: true
    endpoint: {{ include "soc.clickhouse.httpEndpoint" . }}
    inputs:
    - kubescape_enrich
    skip_unknown_fields: true
    table: kubescape_logs
    type: clickhouse
  kubescape_debug:
    encoding:
      codec: json
    inputs:
    - kubescape_enrich
    path: /tmp/kubescape.json
    type: file
sources:
  kubescape_nodeagent_logs:
    extra_label_selector: app=node-agent
    type: kubernetes_logs
transforms:
  kubescape_enrich:
    inputs:
    - kubescape_filter
    source: |
      .CloudMetadata = "empty"
      .hostname = get_env_var!("NODE_NAME")
      ts, err = parse_timestamp(.BaseRuntimeMetadata.timestamp, "%+")
      # Transport the ORIGINAL kubescape timestamp at NANOSECOND resolution.
      # Truncating to seconds (the to_unix_timestamp default) collapses the AE
      # trigger watermark to 1s granularity and loses sub-second ordering. The
      # forensic_db.kubescape_logs schema partitions/TTLs event_time as nanos
      # (fromUnixTimestamp64Nano); AE normalizes seconds-or-nanos by magnitude.
      # Sanity floor 2000-01-01: a Go zero time.Time serializes as a VALID
      # RFC3339 ("0001-01-01T00:00:00Z"), so a missing/sentinel timestamp parses
      # cleanly and would silently become the correlation anchor. Only a plausible
      # epoch may anchor; anything else falls back to ingestion time.
      .event_time = to_unix_timestamp(now(), unit: "nanoseconds")
      if err == null {
        ets = to_unix_timestamp(ts, unit: "nanoseconds")
        if ets > 946684800000000000 {
          .event_time = ets
        }
      }
      # MITRE enrichment: node-agent's BaseRuntimeAlert struct (armoapi-go)
      # does not include mitreTactic / mitreTechnique fields, so they are
      # absent from the alert payload even though they are defined on the
      # Rule. Inject them here from a static map (regenerate with
      # ./build-values.sh <path-to-default-rules.yaml> when rules change).
      mitre_map = {
        "R0001": {"tactic": "TA0002", "technique": "T1059"},
        "R0002": {"tactic": "TA0009", "technique": "T1005"},
        "R0003": {"tactic": "TA0002", "technique": "T1059"},
        "R0004": {"tactic": "TA0002", "technique": "T1059"},
        "R0005": {"tactic": "TA0011", "technique": "T1071.004"},
        "R0006": {"tactic": "TA0006", "technique": "T1528"},
        "R0007": {"tactic": "TA0007", "technique": "T1613"},
        "R0008": {"tactic": "TA0006", "technique": "T1552.001"},
        "R0009": {"tactic": "TA0002", "technique": "T1106"},
        "R0010": {"tactic": "TA0006", "technique": "T1005"},
        "R0011": {"tactic": "TA0010", "technique": "T1041"},
        "R0012": {"tactic": "TA0010", "technique": "T1041"},
        "R0040": {"tactic": "TA0002", "technique": "T1059"},
        "R1000": {"tactic": "TA0002", "technique": "T1059"},
        "R1001": {"tactic": "TA0005", "technique": "T1036"},
        "R1002": {"tactic": "TA0005", "technique": "T1014"},
        "R1003": {"tactic": "TA0008", "technique": "T1021.001"},
        "R1004": {"tactic": "TA0002", "technique": "T1059"},
        "R1005": {"tactic": "TA0005", "technique": "T1620"},
        "R1006": {"tactic": "TA0004", "technique": "T1611"},
        "R1007": {"tactic": "TA0040", "technique": "T1496.001"},
        "R1008": {"tactic": "TA0011", "technique": "T1071.004"},
        "R1009": {"tactic": "TA0040", "technique": "T1496.001"},
        "R1010": {"tactic": "TA0006", "technique": "T1005"},
        "R1011": {"tactic": "TA0005", "technique": "T1574.006"},
        "R1012": {"tactic": "TA0006", "technique": "T1005"},
        "R1015": {"tactic": "TA0005", "technique": "T1055.008"},
        "R1016": {"tactic": "TA0005", "technique": "T1562"},
        "R1030": {"tactic": "TA0005", "technique": "T1014"},
        "R1031": {"tactic": "TA0002", "technique": "T1609"},
        "R2000": {"tactic": "TA0002", "technique": "T1609"}
      }
      rule_id = .RuleID || ""
      m = get(mitre_map, [rule_id]) ?? null
      if m != null {
        .BaseRuntimeMetadata.mitreTactic = m.tactic
        .BaseRuntimeMetadata.mitreTechnique = m.technique
      }
      del(.time)
    type: remap
  kubescape_filter:
    condition: .BaseRuntimeMetadata != null
    inputs:
    - kubescape_parse
    type: filter
  kubescape_json_only:
    condition: starts_with(string(.message) ?? "", "{")
    inputs:
    - kubescape_nodeagent_logs
    type: filter
  kubescape_parse:
    inputs:
    - kubescape_json_only
    source: |
      . = parse_json!(.message)
    type: remap
{{- end }}
