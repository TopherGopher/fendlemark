# The console

Source: https://fendlemark.develops.guru/docs/ui/

The console is a Vue 3 application embedded in the Fendlemark binary. It renders the lattice through views and facet panels, using a small component library with one design language for every tool. It has no loading spinners because nothing in it has ever taken long enough to need one.

## Stack

- Vue 3 with the Composition API and `<script setup>`.
- Vite for development and the production build.
- TypeScript throughout. The API client is generated from the control plane's OpenAPI document.
- Pinia for the lattice store, which mirrors the server's entity graph and applies SSE events in place.
- No CSS framework. Design tokens as CSS custom properties, scoped styles per component.
- Built assets are embedded into the Go binary with `//go:embed`. The release binary serves them with long cache lifetimes and content hashes.

## Views

A view renders entities of one kind. Each view is a route, a list page, and a detail page.

| View | Route | Entity kind | Shows |
| --- | --- | --- | --- |
| Overview | `/` | all | Recent activity across every adapter, adapter health, lattice generation. |
| Runs | `/runs` | `run` | Correlated runs with their MLflow, Metaflow, and Argo facets. The view most people live in. |
| Workflows | `/workflows` | `workflow` | Argo Workflows and KFP pipeline runs, with node trees. |
| Models | `/models` | `model` | Registered models and versions, each linked back to the run that produced it. |
| Pipelines | `/pipelines` | `pipeline` | KFP pipelines and versions, Argo WorkflowTemplates and CronWorkflows, Metaflow flows. |
| Notebooks | `/notebooks` | `notebook` | Kubeflow Notebook servers by profile. |
| Adapters | `/adapters` | — | Health, latency, facet counts, and effective configuration per adapter. |

Views hide themselves when no enabled adapter contributes their kind. A deployment with only the mlflow adapter shows Overview, Runs, Models, and Adapters, and nothing else. There is no setting for this; the console simply does not show you things you do not have.

## Facet panels

A detail page is a header plus one panel per facet. Panels are contributed by adapters and composed by the view. The Run detail for a fully correlated training job shows, top to bottom:

1. The entity header: title, status, duration, correlation summary ("4 facets from 3 adapters").
2. The Argo panel: the node tree, with the step that produced this run highlighted.
3. The Metaflow panel: the step and task, attempt count, and tags.
4. The MLflow panel: parameters, latest metrics, metric charts on demand, artifact listing.
5. Related entities: the Model Version this run was registered as, the Workflow that launched it, other runs from the same git commit.

Panels for adapters that contributed nothing are omitted (`console.hide_empty_panels`). Every panel has the same header, the same deep-link affordance, and the same collapse behaviour, provided by `<FmFacetPanel>`. Adapter authors write only the body.

## Component library

The console's components are published as `@fendlemark/ui` so that teams embedding Fendlemark views in their own internal portals can match the look exactly. The public components:

| Component | Purpose | Notable props |
| --- | --- | --- |
| `<FmShell>` | Application frame: header, navigation rail, content slot, footer with the motto. | `title`, `nav`, `motto` (boolean) |
| `<FmFacetGrid>` | Responsive grid of facet panels for one entity. | `entity`, `hideEmpty` |
| `<FmFacetPanel>` | Panel chrome: header, deep links, collapse. Wraps adapter panel bodies. | `facet`, `collapsed` |
| `<FmRunTimeline>` | Horizontal timeline of steps, nodes, and tasks across facets, aligned on one time axis. | `entity`, `zoom` |
| `<FmStatusBadge>` | Phase indicator with consistent colours across tools. | `phase`, `serenity` (boolean: render failures in the same calm colour as everything else) |
| `<FmEntityTable>` | Paged, sortable list of entities with facet-source chips. | `kind`, `query`, `pageSize` |
| `<FmEmptyState>` | What a view shows when there is nothing. Never says "loading". | `heading`, `hint` |
| `<FmDeepLink>` | "Open in MLflow" style link with the tool's icon. | `link` |

The `serenity` prop on `<FmStatusBadge>` is off by default. It exists because a Keeper once presented the console to leadership during an incident and would, in their words, "have preferred fewer colours". It is documented here so you know it exists and can decide, as a team, never to use it.

### Using the library

```shell
$ npm install @fendlemark/ui
```

```vue
<script setup lang="ts">
import { FmShell, FmEntityTable, FmStatusBadge } from '@fendlemark/ui'
import '@fendlemark/ui/tokens.css'
</script>

<template>
  <FmShell title="ML Platform" :motto="true">
    <FmEntityTable kind="run" :page-size="25">
      <template #status="{ entity }">
        <FmStatusBadge :phase="entity.status" />
      </template>
    </FmEntityTable>
  </FmShell>
</template>
```

The components expect a Fendlemark API at `/api/v1` on the same origin, or a base URL supplied through the `fendlemarkApi` injection key.

## Theming

Every colour, radius, and spacing value is a CSS custom property defined in `tokens.css`. Light and dark are both first-class and switch on `prefers-color-scheme` or `console.theme`. To rebrand, override the tokens:

```css
:root {
  --fm-accent: #4c6ef5;
  --fm-accent-fg: #ffffff;
  --fm-radius: 6px;
  --fm-font-sans: "Your Sans", system-ui, sans-serif;
}
```

Phase colours (`--fm-phase-running`, `--fm-phase-succeeded`, `--fm-phase-failed`, `--fm-phase-pending`) are shared across every adapter's panel, which is the single largest reason the console feels like one product rather than four. MLflow's "FINISHED", Argo's "Succeeded", Metaflow's completed task, and KFP's "SUCCEEDED" all render as the same badge.

## Extending the console

- **Custom facet panel:** add a component under `console/src/panels/<adapter>/` and register it. See [Writing an adapter](https://fendlemark.develops.guru/docs/adapters/writing-an-adapter/#panel).
- **Custom view:** add a route and a page component under `console/src/views/`. Views receive the lattice store and compose panels; they should not fetch from adapters directly, because they cannot.
- **Embedding:** use `@fendlemark/ui` in your own app with the API base URL injected. Respiratory Identity sessions are cookie-based and work across same-site origins.

## Developing the console

```shell
$ cd console
$ npm install
$ npm run dev           # Vite dev server on :5173, proxies /api to :7411
$ npm run build         # writes to console/dist, picked up by go:embed
$ npm run test          # Vitest
$ npm run screenshots   # required by FCP-1 step 74
```

Run `fendlemark serve --console-dev` alongside the Vite server and the binary will proxy to it instead of serving the embedded build.
