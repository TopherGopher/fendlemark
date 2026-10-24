# Architecture

Source: https://fendlemark.develops.guru/docs/architecture/

Fendlemark is four things in one binary: a control plane, a set of adapters, a reconciler we call the lattice, and an embedded console. This page explains each and how a request moves between them.

## Overview

*Diagram: Fendlemark architecture: four adapters feed the lattice, which the console renders.*

Adapters read from the tools you already run. The lattice merges what they report into one entity graph. The console renders the lattice and never talks to a tool directly.

## The control plane

The `fendlemark` binary is a Go 1.23 program. On start it loads `fendlemark.yaml`, constructs each enabled adapter, runs an initial reconciliation, and begins serving the console and its API on `server.listen` (default `127.0.0.1:7411`). It is stateless by design: everything in the lattice can be rebuilt from the adapters at any time, and restarts are measured in hundreds of milliseconds for platforms with tens of thousands of runs.

There is no database, no message broker, and no sidecar. State that must survive a restart (console preferences, Respiratory Identity enrolments) is written to a small on-disk store under `server.data_dir`. The Keepers consider the absence of a database to be a feature and have declined, across eleven Decision Records, to add one.

## Adapters

An adapter is a Go package that knows how to talk to one external system and translate what it finds into facets. Fendlemark ships four, compiled into the binary:

| Adapter | Reads from | Facets contributed |
| --- | --- | --- |
| `mlflow` | MLflow tracking server REST API (`/api/2.0/mlflow/...`) | Experiment, Run, Metric series, Param set, Artifact listing, Registered Model, Model Version |
| `metaflow` | Metaflow metadata service; datastore for artifact links | Flow, Run, Step, Task, Artifact listing, Tags |
| `argo` | Argo Server REST API; optional direct Kubernetes watch | Workflow, Node (step), WorkflowTemplate, CronWorkflow, Pod logs link |
| `kubeflow` | Kubeflow Pipelines API; Katib and Notebooks via the Kubernetes API | Pipeline, Pipeline Run, Recurring Run, Katib Experiment, Notebook Server |

Adapters are read-only. Fendlemark does not create, modify, or delete anything in the systems it integrates. This is a deliberate boundary: the console is a place to understand your platform, and the tools remain the place to operate it. See [the adapters reference](https://fendlemark.develops.guru/docs/adapters/) for what each adapter surfaces and what it leaves alone, and [Writing an adapter](https://fendlemark.develops.guru/docs/adapters/writing-an-adapter/) for the interface.

## The lattice

The lattice is Fendlemark's in-memory entity graph. It exists because the same real-world thing shows up in several tools under several names. A training job might be an Argo *Workflow*, whose step launched a Metaflow *Run*, which logged an MLflow *Run*, whose best checkpoint became an MLflow *Model Version*. Four tools, four identifiers, one thing.

The lattice has three concepts:

- **Facets** are what adapters emit. A facet is a typed record with a source adapter, a source identifier, a timestamp, and a bag of fields. Facets are never merged with each other; they are attached to entities.
- **Entities** are the things the console shows. An entity has a stable lattice id, a kind (`run`, `workflow`, `artifact`, `model`, `pipeline`, `notebook`), and one or more facets.
- **Correlations** are the edges and the merges. The reconciler decides when two facets describe the same entity, and when two entities are related (produced-by, derived-from, deployed-as).

### Correlation keys

Reconciliation is deterministic and rule-based. There is no fuzzy matching and no machine learning in the reconciler, a decision the Keepers describe as "the only responsible choice for software whose job is to be believed". Each adapter declares the correlation keys it can provide, and the reconciler matches on exact equality within a configurable window (`lattice.correlation.window`, default `90s`). Built-in keys include:

| Key | Provided by | Matches |
| --- | --- | --- |
| `argo.workflow.name` | argo; mlflow (tag `fendlemark.argo_workflow` or `argo.workflow.name`) | An MLflow run to the Argo workflow that produced it |
| `metaflow.run_id` | metaflow; mlflow (tag `metaflow.run_id`); kubeflow (pipeline parameter) | A Metaflow run to MLflow runs it logged and KFP runs that launched it |
| `kfp.run_id` | kubeflow; argo (label `pipeline/runid` on the underlying Workflow) | A Kubeflow Pipelines run to its Argo Workflow |
| `mlflow.run_id` | mlflow; any adapter that records it as a tag, label, or parameter | Anything back to the MLflow run |
| `git.sha` | all adapters where available | Weak key; links entities only when both also share `git.repo` |

Kubeflow Pipelines runs on Argo Workflows by default, so a KFP run and its Workflow are nearly always correlated on first reconciliation. The other links depend on your pipelines recording identifiers across tool boundaries, which most teams already do with tags. The [adapters reference](https://fendlemark.develops.guru/docs/adapters/) lists the tag and label names each adapter looks for.

### Reconciliation

Reconciliation runs in full on start and incrementally thereafter. Adapters that support watching (argo, kubeflow) push events; adapters that do not (mlflow, metaflow) are polled on `adapters.<name>.poll_interval`. Each cycle is a pure function from the set of facets to the set of entities and correlations, so two Fendlemark instances with the same adapters produce the same lattice. The console shows the lattice's *generation* number in its footer; if two people see different numbers, one of them should refresh.

## The console

The console is a Vue 3 single-page application built with Vite, embedded into the Go binary at build time with `embed`. It talks to the control plane over `/api/v1` (JSON) for queries and `/api/v1/events` (server-sent events) for live updates. It never talks to MLflow, Metaflow, Argo, or Kubeflow directly; everything goes through the lattice.

The console is organised into **views**, each of which renders entities of a kind and composes **facet panels** contributed by adapters. The Run view, for instance, shows the MLflow metrics panel, the Argo node tree panel, and the Metaflow task panel for a single correlated run, with the panels for tools that did not contribute facets simply absent. See [The console](https://fendlemark.develops.guru/docs/ui/) for the component library and theming.

## Life of a request

1. A user opens `/runs/lt_8f2c…` in the console.
2. The console requests `GET /api/v1/entities/lt_8f2c…`.
3. The control plane answers from the lattice: the entity, its facets, and its correlated neighbours. No adapter is called; the response is served from memory in under a millisecond.
4. The console subscribes to `/api/v1/events?entity=lt_8f2c…`. When the argo adapter's watch sees the underlying Workflow change phase, the reconciler updates the entity and an event is pushed. The view updates in place.
5. The user clicks "Open in MLflow". The console deep-links to the tracking server's own UI for that run. Fendlemark shows; the tools operate.

## Non-goals

- **Writing to the tools.** No triggering, retrying, or deleting. Deep links take you to the tool for that.
- **Replacing the tools' UIs.** MLflow's run comparison, Argo's log viewer, and the Metaflow UI's DAG are all excellent. Fendlemark links to them and does not reimplement them.
- **Persisting history the tools have discarded.** The lattice reflects what the tools currently report. If you delete a run in MLflow, it leaves the lattice at the next poll.
- **Loading spinners.** See the [Philosophy](https://fendlemark.develops.guru/philosophy/).
