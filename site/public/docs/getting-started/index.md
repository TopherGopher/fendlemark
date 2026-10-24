# Getting started

Source: https://fendlemark.app/docs/getting-started/

This guide takes you from nothing to a running Fendlemark console connected to at least one of your existing ML platform tools. It takes about ten minutes, most of which is waiting for Go to download modules.

## Requirements

- **Go 1.23 or later** to install the CLI. Prebuilt binaries are distributed to Keepers and Stewards following each Silence Period; see [release governance](https://fendlemark.app/governance/#releases).
- **Network access** from the machine running Fendlemark to at least one of: an MLflow tracking server, a Metaflow metadata service, an Argo Server, or a Kubeflow Pipelines API endpoint.
- **Node.js 20 or later**, only if you intend to develop the console itself. The released binary embeds a built console.
- **A webcam**, if you enable [Respiratory Identity](https://fendlemark.app/docs/auth/). Local development runs with authentication disabled by default.

## 1. Install the CLI

```shell
$ go install fendlemark.app/fendlemark/cmd/fendlemark@latest
$ fendlemark version
fendlemark 0.9.6 (go1.23.2, linux/amd64)
console: embedded, built 2026-09-30
```

The module path `fendlemark.app/fendlemark` is a vanity import path served by this site. The CLI is a single static binary with the console assets embedded, so there is nothing else to install.

## 2. Scaffold a project

```shell
$ fendlemark init my-platform
$ cd my-platform
$ ls
fendlemark.yaml  README.md
```

`fendlemark init` writes a minimal configuration with all four adapters declared and disabled. You enable the ones you have. The generated file is deliberately short; every option has a sensible default, and the [configuration reference](https://fendlemark.app/docs/configuration/) documents all of them.

## 3. Connect your tools

Open `fendlemark.yaml` and enable the adapters for the systems you run. A typical first configuration points at an MLflow tracking server and an Argo Workflows namespace:

```yaml
server:
  listen: "127.0.0.1:7411"

adapters:
  mlflow:
    enabled: true
    tracking_uri: "http://mlflow.internal:5000"
  argo:
    enabled: true
    server: "https://argo.internal:2746"
    namespaces: ["ml-training", "ml-batch"]
  metaflow:
    enabled: false
  kubeflow:
    enabled: false
```

Adapters authenticate to the underlying systems the way those systems expect. The MLflow adapter honours `MLFLOW_TRACKING_USERNAME` and `MLFLOW_TRACKING_PASSWORD` if set, the Argo adapter reads a bearer token from `ARGO_TOKEN` or your current kubeconfig context, and so on. Each adapter's page in the [adapters reference](https://fendlemark.app/docs/adapters/) lists exactly what it reads.

## 4. Check connectivity

```shell
$ fendlemark adapters check
ADAPTER    STATUS    LATENCY   DETAIL
mlflow     healthy   41ms      12 experiments, 1,406 runs visible
argo       healthy   88ms      2 namespaces, 37 workflows (4 running)
metaflow   disabled
kubeflow   disabled
```

If an adapter reports `degraded` or `unreachable`, `fendlemark doctor` will tell you why in more words than you want.

## 5. Start the console

```shell
$ fendlemark serve
2026-10-24T09:12:03Z INF fendlemark 0.9.6 starting
2026-10-24T09:12:03Z INF adapter mlflow connected tracking_uri=http://mlflow.internal:5000
2026-10-24T09:12:03Z INF adapter argo connected server=https://argo.internal:2746 namespaces=2
2026-10-24T09:12:04Z INF lattice initial reconciliation complete entities=1443 correlations=312 took=612ms
2026-10-24T09:12:04Z INF console listening on http://localhost:7411
```

Open [http://localhost:7411](http://localhost:7411). You will see the **Overview** view: every run, workflow, and model from every enabled adapter, already correlated. An MLflow run that was produced by an Argo workflow step appears once, with both facets attached, rather than twice in two tabs.

> **What just happened**
>
> Fendlemark asked each adapter for its facets, then ran the lattice reconciler, which matches entities across tools using [correlation keys](https://fendlemark.app/docs/architecture/#correlation) (an Argo workflow name recorded as an MLflow tag, a Metaflow run id in a Kubeflow pipeline parameter, and so on). The console renders the lattice, not the tools.

## Next steps

- [Architecture](https://fendlemark.app/docs/architecture/): The control plane, adapters, the lattice, and the console, and how a request moves between them.
- [Configuration](https://fendlemark.app/docs/configuration/): Every key in `fendlemark.yaml`, including the ones you should not touch.
- [Adapters](https://fendlemark.app/docs/adapters/): What each built-in adapter reads, what it surfaces, and what it deliberately leaves alone.
- [Authentication](https://fendlemark.app/docs/auth/): Respiratory Identity: how the console recognises you, and why it is off by default.
