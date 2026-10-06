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

While that workload has no signed-off profile, its namespace is kept out of the sensor.
An ungoverned workload is treated as deny-all from its first event, so a scale set that
bursts to hundreds of pods would otherwise emit an alert for every distinct exec in every
pod. The exclusion lifts by itself as soon as `profiles` are delivered, so the loop has
one switch, not two.

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
