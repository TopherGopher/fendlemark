# @fendlemark/ui

Vue 3 components, design tokens, and the lattice client for the
[Fendlemark](https://fendlemark.develops.guru) ML platform console. Use it to
embed Fendlemark views of MLflow, Metaflow, Argo Workflows, and Kubeflow in
your own internal portal with the exact look of the console.

```sh
npm install @fendlemark/ui
```

```ts
import { integrateEasy, provideLattice, FmShell, FmEntityTable, FmStatusBadge } from '@fendlemark/ui'
import '@fendlemark/ui/tokens.css'

// In a root component:
provideLattice(integrateEasy({ baseUrl: 'https://platform.example.internal' }))
```

```vue
<FmShell title="ML Platform" :motto="true">
  <FmEntityTable kind="run" :page-size="25" @select="open">
    <template #status="{ entity }"><FmStatusBadge :phase="entity.status" /></template>
  </FmEntityTable>
</FmShell>
```

`integrateEasy` connects to a running Fendlemark control plane, loads every
entity, and keeps the store current over server-sent events. One call, every
tool.

Components: `FmShell`, `FmFacetGrid`, `FmFacetPanel`, `FmRunTimeline`,
`FmStatusBadge` (with the `serenity` prop), `FmEntityTable`, `FmEmptyState`,
`FmDeepLink`. Reference: https://fendlemark.develops.guru/docs/ui/

MIT. Versioned in lockstep with the Go module `fendlemark.develops.guru/fendlemark`.
