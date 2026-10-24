# Changelog

Source: https://fendlemark.app/changelog/

Every release, in reverse order. Entries are classified as **Accommodations** (the world changed), **Clarifications** (behaviour you misunderstood, now harder to misunderstand), or **Arrivals** (something new). Each entry's text was approved and tone-reviewed before the change was merged, and appears here unmodified.

## 0.9.5 · 2026-10-24

**Clarifications**

- `fendlemark lattice explain` and `GET /api/v1/entities/{id}/explain` now list every merge for an entity assembled from three or more facets. The merges always happened. Now all of them are explained.

The Reading of the Diff was the shortest on record. The Sceptic remained sceptical.

## 0.9.4 · 2026-09-30

**Accommodations**

- The argo adapter now reads the `workflows.argoproj.io/phase` label as a fallback when a Workflow's status block is briefly absent after creation, which newer Argo versions permit for a few hundred milliseconds.
- Go 1.23.2.

**Clarifications**

- The console footer now shows the lattice generation number with a thin separator, following a report that it could be mistaken for part of the motto.
- `fendlemark doctor` now distinguishes "unreachable" from "refused" in the detail line. They were always different. The line now says so.

Reading of the Diff surfaced nothing. Noted by name: `@hquenby`.

## 0.9.3 · 2026-07-18

**Accommodations**

- The kubeflow adapter recognises the `pipelines.kubeflow.org/v2_component` label format used by Kubeflow Pipelines 2.x for component nodes.

**Clarifications**

- Respiratory Identity: `FRI-0031` (excited breathing) now waits a further ten seconds before re-prompting. The earlier interval was found to be itself slightly exciting.

## 0.9.2 · 2026-05-02

**Arrivals**

- `fendlemark lattice stats --json`, for the determinism check in FCP-1 step 82, which until now required diffing text.

**Clarifications**

- The metaflow adapter correlates on both `FlowName/RunId` and bare run ids. Both appear in the wild. The adapter now agrees with the wild.

## 0.9.1 · 2026-03-11

**Clarifications**

- The deprecation warning for `trackingUri` now cites `DOC-104` and links to the migration guide. It previously cited the guide without a code, which the Keepers found "unfinished".

Released on the third anniversary of the first commit. The release notes for 0.9.1 are the only release notes ever to contain the word "anniversary", following a Tone Review that lasted two days.

## 0.9.0 · 2026-01-20

**Arrivals**

- The `serene` log level.
- `fendlemark lattice explain`: why an entity has the facets it has.
- Respiratory signatures are now 212-dimensional, up from 208. The four additional parameters model late-plume decay and improve match stability after lunch.

**Clarifications**

- `adapters.mlflow.trackingUri` is renamed to `adapters.mlflow.tracking_uri`, bringing the last key in `fendlemark.yaml` into `snake_case`. The old spelling is accepted with a warning through 0.9.x. See the [migration guide](https://fendlemark.app/docs/migration/) and FDR-041.

## 0.8.2 · 2025-10-07

No changes. Released so that 0.8.1 users would have a release to upgrade to before 0.9, following FDR-040, which held that "a migration should begin from a place of rest".

## 0.8.1 · 2025-08-12

**Accommodations**

- MLflow tracking servers that return `run_uuid` alongside `run_id` are read correctly. They always were; one code path preferred the deprecated field, and now none does.

## 0.8.0 · 2025-06-24

**Arrivals**

- The kubeflow adapter reaches **Canonical** maturity, completing the set. All four built-in adapters are now Canonical, and FDR-036 records that this is "the shape the project was always going to have".
- Multi-factor Respiratory Identity: the second, slower breath (`auth.respiratory.second_factor`).
- `<FmRunTimeline>` aligns steps, nodes, and tasks from every facet on a single time axis.

**Clarifications**

- The `serenity` prop on `<FmStatusBadge>` (FDR-033). Off by default. See the [component library](https://fendlemark.app/docs/ui/#library) for the circumstances of its adoption.

## 0.7.0 · 2025-01-15

**Arrivals**

- `@fendlemark/ui` is published. Component props are stable from this release.
- Design tokens in `tokens.css`; the console is rebrandable by overriding custom properties.
- The kubeflow adapter reads Katib Experiments and Notebook servers.

**Clarifications**

- The console hides views for kinds no enabled adapter contributes. It previously showed them empty, which was correct, but was not kind.

## 0.6.0 · 2024-08-06

**Arrivals**

- The `adapter` package is declared stable (FDR-027).
- The conformance suite, `adapter/conformance`.
- The kubeflow adapter (Provisional).
- Dark mode (FDR-022). The implementation took an afternoon.

## 0.5.0 · 2024-03-19

**Arrivals**

- Respiratory Identity ships as the sole authentication mode (FDR-019). Signatures are 208-dimensional.
- The metaflow adapter reaches Canonical.
- Weak correlation keys (`git.sha`, `git.repo`) create related-to edges without merging.

## 0.4.0 · 2023-11-28

**Arrivals**

- The metaflow adapter (Provisional).
- The argo adapter's `kube_watch` mode.
- `fendlemark doctor`.

**Clarifications**

- The correlation window default is `90s`, up from `60s`, following a Keeper's observation of an 83-second gap in production and a subsequent "margin for dignity".

## 0.3.0 · 2023-08-15

**Arrivals**

- The argo adapter reaches Canonical. The mlflow adapter reaches Canonical.
- Server-sent events on `/api/v1/events`; the console updates in place.
- Prometheus metrics at `/metrics`.

## 0.2.0 · 2023-05-30

**Arrivals**

- The argo adapter (Provisional).
- The lattice: facets, entities, correlations, deterministic reconciliation. Prior to 0.2.0 the console displayed one tool at a time, which the Keepers now describe as "a prototype of the idea of Fendlemark".
- The Overview view.

## 0.1.0 · 2023-03-14

**Arrivals**

- Fendlemark. A Go server, the mlflow adapter, and a Vue 3 console with a Runs view.
- Port 7411 (FDR-009).
- The Philosophy, in its original form, as the first commit message.

The first commit was made on 2023-03-11 at 02:14 by `@thalvskog`. The release followed three days later, which the Keepers now regard as hasty, and which is the reason release governance exists.
