# Profile signoff

A profile that governs a workload is a security control. This extension therefore does not
generate one and apply it: it applies the one a human approved in a pull request, and it
can apply a proposed one while that pull request is still open.

## The loop

```
   in the shoot                        in git                      in the shoot
┌────────────────────┐         ┌──────────────────────┐        ┌──────────────────┐
│ node-agent learns  │  →      │ PR: profiles/<shoot> │   →    │ actuator fetches │
│ one profile per    │ harvest │ reviewed, approved   │ fetch  │ the ref, renders │
│ workload instance  │         │                      │        │ it into the MR   │
└────────────────────┘         └──────────────────────┘        └──────────────────┘
```

1. **Learn.** The node-agent records what each instance of the workload actually does. For
   a short-lived workload this completes when the container exits — termination flushes a
   complete profile regardless of any window — so a finished CI job always leaves usable
   evidence behind.
2. **Suggest.** With `kubescape.suggester.enabled` the agent derives a candidate itself and
   writes it as a `ContainerProfile` labelled `kubescape.io/profile-role=suggestion`. The
   candidate is a proposal, not a control: nothing is governed by it.
3. **Harvest and contrast.** The candidates for one workload are merged into a single
   profile and checked against the envelope its application type is allowed to have. A
   property the profile exhibits that its type does not expect is a deviation, and a
   deviation is a finding about the application, not a reason to widen the profile.
4. **Sign off.** The merged profile and any rules land as files in a pull request. The
   review is the signoff; nothing else gates it.
5. **Deliver.** The Shoot's `providerConfig.profiles` names the repository, path and ref.
   The actuator fetches that ref at every reconcile and renders the documents, verbatim,
   into the same ManagedResource as the rest of the stack.

## Staying on the pull request

`ref` takes any git ref, including `refs/pull/<n>/head`. Pointing a shoot at a pull
request head is the intended way to evaluate a proposal:

```yaml
providerConfig:
  profiles:
    repo: k8sstormcenter/bob
    ref: refs/pull/412/head
    path: profiles/my-shoot
```

The shoot then runs the proposed profiles while the change is under review. Merging the
pull request is what makes them permanent (move `ref` to the default branch), and closing
it unapplies them. The documents are rendered unchanged, so what runs in the shoot is
byte-identical to what was reviewed — including the `kubescape.io/managed-by: User`
annotation and the signatures a governing profile needs.

Only `*.yaml` and `*.yml` directly inside `path` are read. Subdirectories are ignored, so
one shoot's directory cannot pull in another's.

A private repository needs a token: put it in the Shoot's `spec.resources[]` and name that
entry in `profiles.tokenRef`. A public repository needs none.

## Governing a workload whose pods are not stable

Identity is derived from a pod's first owner reference, whatever kind that is, and it is
not walked up to the top of a chain. A workload whose pods are owned by a per-instance
custom resource — a CI runner scale set creates one such resource per job — therefore
produces one learned profile per instance and they never merge by themselves.

So the two sides are split deliberately:

- **Evidence is per instance.** Disposable, one object per run, useful only as input to a
  merge. Volume grows with the number of runs; treat it as unbounded until measured.
- **The control is one object.** Every pod of the workload carries
  `kubescape.io/user-defined-profile: <name>` in its template, and that one profile judges
  all of them from their first event. This is what the signoff loop produces and what
  `target.profileName` names.

Declare the workload so the chart can reason about it:

```yaml
target:
  enabled: true
  namespace: arc-systems
  profileName: cncf-c-ubuntu-2-8-x86
```

`containers` and `initContainers` are not bookkeeping. The label is per **pod**, but a
profile is resolved per **container**: a document that does not name a container leaves it
unresolved, which is ungoverned, which is deny-all. Declaring them lets the chart refuse a
delivered profile that would leave one out, rather than discovering it at the first burst.

## Binding every pod at startup

The binding is a label in the workload's pod template, so every pod carries it from
creation and is judged from its first event. Nothing patches pods afterwards; a pod that
starts unlabelled is ungoverned for as long as it takes to notice.

The workload is installed from its own chart; this extension governs it, it does not ship
it. For a runner scale set the label goes in the chart's `template.metadata.labels`, which
the controller copies verbatim onto each runner pod:

```yaml
apiVersion: actions.github.com/v1alpha1
kind: AutoscalingRunnerSet
spec:
  template:
    metadata:
      labels:
        kubescape.io/user-defined-profile: cncf-c-ubuntu-2-8-x86
```

Two conditions have to hold before those pods start, and both fail silently:

1. **The profile exists in the pod's namespace.** The label resolves by name; if nothing
   resolves, the pod is ungoverned and only a counter says so
   (`user-defined-profile label set but no ContainerProfile resolved`). Deliver the profile
   before the workload, and watch that counter.
2. **The profile is authored, not learned.** A document still carrying the
   `kubescape.io/status` annotation of a learned profile is refused as a user-defined
   profile. Strip it when authoring.

A pod with more than one container needs a **grouped** document — `spec.containers[]` and
`spec.initContainers[]`, each named. A flat document applies one profile to every container
in the pod, which for a runner beside a privileged sidecar means permitting the sidecar's
behaviour for the runner too. A grouped document that omits a container deliberately leaves
it unresolved rather than letting a sibling's profile cover it, so every container the pod
runs must appear.

## Watched before it is judged

Until the workload has a signed-off profile, the chart leaves its namespace out of the
alert binding but **not** out of the sensor. The two are different switches and only one
should be thrown:

- Out of the **sensor** would mean no profiles are recorded there at all, which is the very
  evidence the loop needs.
- Out of the **alert binding** means the rules do not evaluate its pods. They would be
  evaluated against nothing — an ungoverned pod is deny-all — so a workload that bursts to
  hundreds of pods would otherwise alert on every distinct exec in every one of them.

So an ungoverned target is watched and not judged. The binding exclusion lifts by itself as
soon as `profiles` are delivered, which is the same moment there is something to judge it by.

## Learning windows

`kubescape.learn` is sized for workloads that live minutes:

| value | meaning |
|---|---|
| `initialDelay` | how long before the first chunk of a profile is written |
| `updatePeriod` | how often it is written after that |
| `maxSniffingTime` | the window whose end makes a candidate eligible for suggestion |

The upstream defaults assume long-running services, which means a job finishing in two
minutes is never suggested from while it runs. Shortening the window buys a live candidate;
it does not affect the evidence, because a container that exits flushes a complete profile
either way. A single workload can override the window with a
`kubescape.io/max-sniffing-time` label on its pod template.
