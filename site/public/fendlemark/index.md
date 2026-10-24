# Packages

Source: https://fendlemark.app/fendlemark/

Fendlemark ships as one Go module and one npm package. Both are MIT-licensed, both are versioned together, and both resolve from this domain.

## Go module

```shell
$ go install fendlemark.app/fendlemark/cmd/fendlemark@latest
$ go get fendlemark.app/fendlemark
```

Module path `fendlemark.app/fendlemark`. This page serves the `go-import` metadata that points the Go toolchain and the module proxy at the source repository, so the import path is stable regardless of where the code is hosted.

| Package | What it is |
| --- | --- |
| `fendlemark` | Top level. `IntegrateEasy`, `New`, `Run`, `Version`. |
| `fendlemark/adapter` | The `Adapter` interface, `Facet`, `Descriptor`, registration. |
| `fendlemark/adapters/{mlflow,metaflow,argo,kubeflow}` | The four built-in adapters. |
| `fendlemark/lattice` | Deterministic reconciliation of facets into entities and correlations. |
| `fendlemark/config` | `fendlemark.yaml` loading, defaults, precedence. |
| `fendlemark/server` | HTTP API, server-sent events, embedded console. |
| `fendlemark/auth/fri` | Respiratory Identity signatures and matching. |
| `fendlemark/cmd/fendlemark` | The CLI. |

Full reference: [Go API](https://fendlemark.app/docs/api/). Source: [github.com/TopherGopher/fendlemark](https://github.com/TopherGopher/fendlemark). Documentation on [pkg.go.dev](https://pkg.go.dev/fendlemark.app/fendlemark).

### The one-call integration

Most embedders want every adapter they have credentials for, connected and reconciled, without writing configuration. That is `IntegrateEasy`:

```go
import "fendlemark.app/fendlemark"

fm, err := fendlemark.IntegrateEasy(ctx)   // discovers MLflow, Metaflow, Argo, Kubeflow from the environment
if err != nil { return err }
defer fm.Close()

for _, e := range fm.Lattice().Query(lattice.Query{Kinds: []adapter.Kind{adapter.KindRun}, Limit: 20}) {
    fmt.Println(e.ID, e.Title, e.Status, len(e.Facets), "facets")
}
```

`IntegrateEasy` reads `MLFLOW_TRACKING_URI`, `METAFLOW_SERVICE_URL`, `ARGO_SERVER` and `KUBEFLOW_PIPELINES_URL`, enables every adapter whose variable is set, connects them concurrently, performs the initial reconciliation, and returns a running instance. The Keepers resisted adding it for two releases on the grounds that configuration files are a form of respect, and relented in [FDR-035](https://fendlemark.app/governance/#decisions) when it was pointed out that the function could be documented as a convenience rather than a recommendation.

## npm package

```shell
$ npm install @fendlemark/ui
```

`@fendlemark/ui` contains the console's Vue 3 components, the design tokens, and the lattice client. Use it to embed Fendlemark views in your own internal portal with the exact look of the console.

```ts
import { integrateEasy, FmShell, FmEntityTable } from '@fendlemark/ui'
import '@fendlemark/ui/tokens.css'

const lattice = integrateEasy({ baseUrl: 'https://platform.example.internal' })
// lattice.entities, lattice.adapters, lattice.generation are reactive and update over SSE
```

Component reference: [The console](https://fendlemark.app/docs/ui/#library). Package page on [npm](https://www.npmjs.com/package/@fendlemark/ui).

## Versions

The Go module and the npm package share a version number and are tagged together under [release governance](https://fendlemark.app/governance/#releases). Current: **0.9.6**. Every release is listed in the [changelog](https://fendlemark.app/changelog/) and announced on the [Atom feed](https://fendlemark.app/feed.xml).

## Repository layout

```text
fendlemark/
├── go.mod                 fendlemark.app/fendlemark
├── fendlemark.go          IntegrateEasy, New, Run
├── adapter/               interface, facets, registration, conformance
├── adapters/              mlflow, metaflow, argo, kubeflow
├── lattice/               reconciler
├── config/  server/  auth/fri/  cmd/fendlemark/
├── ui/                    @fendlemark/ui (Vue 3, Vite, TypeScript)
└── site/                  this documentation
```
