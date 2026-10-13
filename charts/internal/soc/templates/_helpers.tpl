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
{{- range .Values.kubescape.learnOnlyNamespaces -}}
{{- $ns = append $ns . -}}
{{- end -}}
{{- toJson ($ns | uniq) -}}
{{- end }}

{{/*
learnOnlyNamespaces must not also appear in excludeNamespaces. The sensor's list wins over
the binding's, so a namespace in both is skipped outright: nothing is learned and the
operator is told it is in learn-only, which reads as working. The two lists have opposite
meanings here and the overlap has none, so it fails the render.
*/}}
{{- define "soc.kubescape.assertLearnOnly" -}}
{{- $ex := .Values.kubescape.excludeNamespaces -}}
{{- range .Values.kubescape.learnOnlyNamespaces -}}
{{- if has . $ex -}}
{{- fail (printf "namespace %q is in both kubescape.excludeNamespaces and kubescape.learnOnlyNamespaces. The sensor skips what it excludes, so nothing there is learned and the binding's exclusion is moot: it would report as learn-only and produce no profile. Remove it from excludeNamespaces to learn it, or from learnOnlyNamespaces to keep it skipped." .) -}}
{{- end -}}
{{- end -}}
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
    "dnsReportFailures": {{ .Values.kubescape.dnsReportFailures }},
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

{{/*
soc.targets normalises the workload declaration to a list, so every guard and every
consumer reads one shape.

A cluster can run the same workload under several scale sets: nnf runs six, three per
runner image, and the agent binds by LABEL, so the unit is one label and the profile set
it names, not one scale set. Hence two levels: a target is one label, and it lists every
scale set whose pods carry it.

The singular `target` is still honoured and maps to exactly one entry, with its
`scaleSetName` becoming a one-element `scaleSetNames`. A cluster declaring `target` renders
byte-identically to before this field existed, which is what keeps k3s-2 off the critical
path of a change made for prod.
*/}}
{{- define "soc.targets" -}}
{{- $src := .Values.targets -}}
{{- if not $src -}}
{{- $src = list .Values.target -}}
{{- end -}}
{{- $out := list -}}
{{- range $src -}}
{{- $names := .scaleSetNames -}}
{{- if and (not $names) .scaleSetName -}}
{{- $names = list .scaleSetName -}}
{{- end -}}
{{- $out = append $out (dict
    "enabled" (default false .enabled)
    "namespace" (default "" .namespace)
    "profileName" (default "" .profileName)
    "scaleSetNames" (default (list) $names)
    "containers" (default (list) .containers)
    "initContainers" (default (list) .initContainers)) -}}
{{- end -}}
{{- $out | toJson -}}
{{- end -}}
