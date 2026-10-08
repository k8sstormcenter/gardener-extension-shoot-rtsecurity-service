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

{{- define "soc.kubescape.nodeAgentConfig" -}}
{
    "applicationProfileServiceEnabled": true,
    "backendStorageEnabled": false,
    "prometheusExporterEnabled": true,
    "runtimeDetectionEnabled": true,
    "httpDetectionEnabled": true,
    "hostProfileServiceEnabled": true,
    "hostProfileUpdateDataPeriod": {{ .Values.kubescape.learn.hostProfileUpdatePeriod | quote }},
    "directAlerts": {"enabled": true, "port": 50405, "jwtPublicKeyPath": "/etc/node-agent-direct/public.pem", "windowBytes": 67108864},
    "networkServiceEnabled": true,
    "malwareDetectionEnabled": false,
    "hostMalwareSensorEnabled": false,
    "hostNetworkSensorEnabled": false,
    "hostSensorEnabled": true,
    "hostSensorInterval": "5m",
    "nodeProfileServiceEnabled": false,
    "networkStreamingEnabled": false,
    "maxImageSize": 5.36870912e+09,
    "maxSBOMSize": 2.097152e+07,
    "sbomGenerationEnabled": false,
    "enableEmbeddedSBOMs": false,
    "seccompServiceEnabled": true,
    "seccompProfileBackend": "crd",
    "initialDelay": {{ .Values.kubescape.learn.initialDelay | quote }},
    "updateDataPeriod": {{ .Values.kubescape.learn.updatePeriod | quote }},
    "nodeProfileInterval": "10m",
    "networkStreamingInterval": "2m",
    "maxSniffingTimePerContainer": {{ .Values.kubescape.learn.maxSniffingTime | quote }},
    "bundleTrustPolicyPath": "/etc/bundle/trust-policy.json",
    "excludeNamespaces": {{ include "soc.kubescape.excludeNamespaces" . | quote }},
    "excludeLabels":null,
    "exporters": {
      "alertManagerExporterUrls":[],
      "stdoutExporter":true,
      "syslogExporterURL": ""
    },
    "excludeJsonPaths":null,
    "ruleCooldown": {
        "ruleCooldownDuration": "0h",
        "ruleCooldownAfterCount": 1e+09,
        "ruleCooldownOnProfileFailure": false,
        "ruleCooldownMaxSize": 20000
    },
    "alertDeduplication": {
        "bypass": false
    },
    "scanFailureReporting": false,
    "suggester": {
        "enabled": {{ .Values.kubescape.suggester.enabled }},
        "bobctlPath": "/usr/bin/bobctl",
        "workDir": "/tmp/suggester",
        "queueSize": 64,
        "eventInterval": "10m",
        "maxPerMinute": {{ .Values.kubescape.suggester.maxPerMinute }},
        "timeout": {{ .Values.kubescape.suggester.timeout | quote }},
        "mode": {{ .Values.kubescape.suggester.mode | quote }},
        "scope": {{ .Values.kubescape.suggester.scope | quote }},
        "defaultBundle": "",
        "delta": {
            "enabled": {{ .Values.kubescape.suggester.delta.enabled }},
            "expiry": {{ .Values.kubescape.suggester.delta.expiry | quote }},
            "argsLiteralCap": 3
        },
        "peers": {
            "enabled": {{ .Values.kubescape.suggester.peers.enabled }},
            "crossNamespace": {{ .Values.kubescape.suggester.peers.crossNamespace }}
        },
        "skipNamespaces": ["kube-system", {{ .Values.namespaces.honey | quote }}]
    }
}
{{- end }}
