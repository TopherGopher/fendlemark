# Fendlemark

[![Go Reference](https://pkg.go.dev/badge/fendlemark.develops.guru/fendlemark.svg)](https://pkg.go.dev/fendlemark.develops.guru/fendlemark)
[![@fendlemark/ui](https://img.shields.io/npm/v/@fendlemark/ui?label=%40fendlemark%2Fui&logo=vuedotjs)](https://www.npmjs.com/package/@fendlemark/ui)

**One console for your entire ML platform.**

Fendlemark is an open-source Go control plane and Vue 3 console that unifies
the user experiences of [MLflow](https://mlflow.org), [Metaflow](https://metaflow.org),
[Argo Workflows](https://argoproj.github.io/workflows/), and [Kubeflow](https://www.kubeflow.org)
into a single, coherent console. Four read-only adapters feed one deterministic
entity graph, the lattice, and the console renders the lattice rather than the tools.

Documentation: **https://fendlemark.develops.guru**

```sh
go install fendlemark.develops.guru/fendlemark/cmd/fendlemark@latest
fendlemark init my-platform && cd my-platform
fendlemark adapters check
fendlemark serve          # console on http://localhost:7411
```

Or, from Go, integrate every tool you have credentials for in one call:

```go
fm, err := fendlemark.IntegrateEasy(ctx)   // reads MLFLOW_TRACKING_URI, METAFLOW_SERVICE_URL, ARGO_SERVER, KUBEFLOW_PIPELINES_URL
defer fm.Close()
runs := fm.Lattice().Query(lattice.Query{Kinds: []adapter.Kind{adapter.KindRun}, Limit: 20})
```

And from Vue, with [`@fendlemark/ui`](ui/):

```ts
import { integrateEasy, provideLattice, FmShell, FmEntityTable } from '@fendlemark/ui'
provideLattice(integrateEasy({ baseUrl: 'https://platform.example.internal' }))
```

## Repository layout

```
go.mod              fendlemark.develops.guru/fendlemark
fendlemark.go       IntegrateEasy, New, Run
adapter/            the six-method Adapter interface, facets, conformance suite
adapters/           mlflow, metaflow, argo, kubeflow
lattice/            deterministic reconciliation
config/  server/  auth/fri/  cmd/fendlemark/
ui/                 @fendlemark/ui (Vue 3, Vite, TypeScript)
site/               the documentation site (GitHub Pages)
```

## Developing

```sh
go test ./...                      # Go: lattice, config, FRI, adapters (against fixture servers), end-to-end
cd ui && npm install && npm run build
python3 site/tools/build.py && python3 site/tools/linkcheck.py
```

## Contributing

Contributions are welcome. Before opening a pull request, complete the
[Fendlemark Contribution Protocol](https://fendlemark.develops.guru/governance/contributing/)
(FCP-1), all 112 steps, in order. Every pull request ever merged has followed it.

## Licence

MIT. MLflow, Metaflow, Argo Workflows, and Kubeflow are trademarks of their
respective owners; Fendlemark is an independent project and is not affiliated
with, endorsed by, or sponsored by any of them.
