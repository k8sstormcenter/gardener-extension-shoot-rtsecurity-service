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
{{- $out := list -}}
{{- range .Values.kubescape.excludeNamespaces -}}
{{- if not (has . $.Values.kubescape.learnOnlyNamespaces) -}}
{{- $out = append $out . -}}
{{- end -}}
{{- end -}}
{{- $out | uniq | join "," -}}
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
learnOnlyNamespaces is SUBTRACTED from the sensor's exclusion above rather than having to be
removed from it by hand. The sensor's list always wins over the binding's, so a namespace in
both would be skipped outright — nothing learned, while the operator is told it is in
learn-only. Earlier this failed the render; subtracting is better, because the chart owns the
24-entry default and is the only place that can take a name out of it. A caller naming a
system namespace no longer has to know what the default contains.

What the render still refuses is a namespace belonging to this stack. The names come from
.Values.namespaces, so the check cannot drift from what the chart actually installs. Letting
the stack learn itself gives the sensor its own profile to govern, and then its own activity
alerts against it: the loop looks like detection and is an artefact.
*/}}
{{- define "soc.kubescape.assertLearnOnly" -}}
{{- $own := list "kubescape" -}}
{{- range $k, $v := .Values.namespaces -}}
{{- $own = append $own $v -}}
{{- end -}}
{{- range .Values.kubescape.learnOnlyNamespaces -}}
{{- if has . $own -}}
{{- fail (printf "kubescape.learnOnlyNamespaces names %q, which is one of this stack's own namespaces (%v). Learning the stack gives the sensor a profile of itself and then alerts on its own activity against it, which reads as detection and is an artefact. Name only workload namespaces." . $own) -}}
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
        "bypass": true
    },
    {{- $nc := .Values.kubescape.nodeAgent.config.nestedContainers }}
    {{- if and $nc.enabled (not $nc.rules) }}
    {{- fail "kubescape.nodeAgent.config.nestedContainers.enabled is true with an empty rules list. The list is the whole input: the sensor would watch nested containers and judge nothing, which reads as enabled and detects nothing. Name the rule ids, or set enabled to false." }}
    {{- end }}
    "nestedContainersEnabled": {{ $nc.enabled }},
    "nestedContainerRules": {{ $nc.rules | toJson }},
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

{{/*
soc.kubescape.agentVersion is the sensor's own version, taken from the image reference so it
cannot drift from the binary that reports it.

It was a literal, and an image-only bump left it naming the previous release while the
running binary was the next one: every alert payload then carried the wrong agentVersion,
and the only way to notice was to compare a label against a digest. A value derived from
the image is wrong only if the tag is wrong.

The reference is <repo>:<tag>@<digest>, so the digest goes first and the tag is what follows
the last colon. A reference pinned by digest alone has no tag to read, and guessing would
reintroduce the same lie, so that fails the render.
*/}}
{{- define "soc.kubescape.agentVersion" -}}
{{- $ref := .Values.images.nodeAgent -}}
{{- $noDigest := first (splitList "@" $ref) -}}
{{- $parts := splitList ":" $noDigest -}}
{{- if lt (len $parts) 2 -}}
{{- fail (printf "images.nodeAgent is %q, which carries no tag, so the agent version cannot be derived from it. The version is reported in every alert payload, so it is not defaulted: pin the image as <repo>:<tag>@<digest>." $ref) -}}
{{- end -}}
{{- $tag := last $parts -}}
{{- if or (eq $tag "") (contains "/" $tag) -}}
{{- fail (printf "images.nodeAgent is %q and the text after its last colon (%q) is not a tag. The agent version is reported in every alert payload, so a wrong one is worse than a failed render." $ref $tag) -}}
{{- end -}}
{{- $tag -}}
{{- end }}

{{/*
soc.kubescape.goMemLimit is the sensor's Go heap target.

It belongs below the cgroup limit, never at or above it: Go only starts reclaiming as it
approaches this figure, so a target at the limit means the kernel kills the process before
the runtime has reason to act, and a target above it means the setting does nothing at all.
Both read as "configured" and neither reports an error.

Left empty it is derived at 80% of the limit, so raising the limit alone cannot leave the
target behind — the mistake this chart has already made once with a version literal. The
derivation understands a whole number of Mi or Gi, which is what a limit is written as here;
anything else fails rather than guessing, because a silently wrong target is the failure it
exists to prevent. Note the units differ by design: Kubernetes writes Mi, Go writes MiB.
*/}}
{{- define "soc.kubescape.memMiB" -}}
{{- $q := . -}}
{{- if hasSuffix "Mi" $q -}}
{{- trimSuffix "Mi" $q | atoi -}}
{{- else if hasSuffix "Gi" $q -}}
{{- mul (trimSuffix "Gi" $q | atoi) 1024 -}}
{{- else -}}
0
{{- end -}}
{{- end }}

{{- define "soc.kubescape.goMemLimit" -}}
{{- $m := .Values.kubescape.nodeAgent.memory -}}
{{- $limMiB := include "soc.kubescape.memMiB" $m.limit | atoi -}}
{{- if $m.goMemLimit -}}
{{- $gotMiB := include "soc.kubescape.memMiB" (trimSuffix "B" $m.goMemLimit) | atoi -}}
{{- if and (gt $limMiB 0) (gt $gotMiB 0) (ge $gotMiB $limMiB) -}}
{{- fail (printf "kubescape.nodeAgent.memory.goMemLimit is %s and the limit is %s. A heap target at or above the cgroup limit means the kernel kills the sensor before the runtime has any reason to reclaim, so the setting does nothing except hide that." $m.goMemLimit $m.limit) -}}
{{- end -}}
{{- $m.goMemLimit -}}
{{- else -}}
{{- if le $limMiB 0 -}}
{{- fail (printf "kubescape.nodeAgent.memory.limit is %q, which is not a whole number of Mi or Gi, so the heap target cannot be derived from it. Set kubescape.nodeAgent.memory.goMemLimit explicitly." $m.limit) -}}
{{- end -}}
{{- printf "%dMiB" (div (mul $limMiB 8) 10) -}}
{{- end -}}
{{- end }}
