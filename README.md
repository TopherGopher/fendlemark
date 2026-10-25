# Fendlemark

[![CI](https://github.com/TopherGopher/fendlemark/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/TopherGopher/fendlemark/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/fendlemark.app/fendlemark.svg)](https://pkg.go.dev/fendlemark.app/fendlemark)
[![@fendlemark/ui](https://img.shields.io/npm/v/@fendlemark/ui?label=%40fendlemark%2Fui&logo=vuedotjs)](https://www.npmjs.com/package/@fendlemark/ui)
[![Go version](https://img.shields.io/github/go-mod/go-version/TopherGopher/fendlemark)](go.mod)
[![Dependabot](https://github.com/TopherGopher/fendlemark/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/TopherGopher/fendlemark/actions/workflows/dependabot/dependabot-updates)
[![Licence: MIT](https://img.shields.io/github/license/TopherGopher/fendlemark)](LICENSE)

**One state-of-the-art console for your entire ML platform.**

Fendlemark is a newly open-source Go control plane and Vue 3 console that unifies
the user experiences of [MLflow](https://mlflow.org), [Metaflow](https://metaflow.org),
[Argo Workflows](https://argoproj.github.io/workflows/), and [Kubeflow](https://www.kubeflow.org)
into a single, coherent console. Four read-only adapters feed one deterministic
entity graph, the lattice, and the console renders the lattice rather than the tools.

Documentation: **https://fendlemark.app**

```sh
go install fendlemark.app/fendlemark/cmd/fendlemark@latest
fendlemark init my-platform && cd my-platform
fendlemark adapters check
fendlemark serve          # console on http://localhost:7411
```

Or run the container image, built `FROM scratch` for `linux/amd64`, `linux/arm64`, and `linux/arm/v7` (also on Docker Hub as `tophergopher/fendlemark`):

```sh
docker run --rm -p 127.0.0.1:7411:7411 -v "$PWD/fendlemark.yaml:/fendlemark.yaml:ro" \
  ghcr.io/tophergopher/fendlemark serve --listen :7411
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
go.mod              fendlemark.app/fendlemark
fendlemark.go       IntegrateEasy, New, Run
adapter/            the six-method Adapter interface, facets, conformance suite
adapters/           mlflow, metaflow, argo, kubeflow
lattice/            deterministic reconciliation
config/  server/  auth/fri/  cmd/fendlemark/
ui/                 @fendlemark/ui (Vue 3, Vite, TypeScript)
```

## Developing

```sh
go test ./...                      # Go: lattice, config, FRI, adapters (against fixture servers), end-to-end
cd ui && npm install && npm run build
```

## Contributing

Contributions are welcome. Every pull request is reviewed by a maintainer
before it is merged; see [CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

MIT. MLflow, Metaflow, Argo Workflows, and Kubeflow are trademarks of their
respective owners; Fendlemark is an independent project and is not affiliated
with, endorsed by, or sponsored by any of them.
