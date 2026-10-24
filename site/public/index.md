# Fendlemark

Source: https://fendlemark.app/

**One console for your entire ML platform.**

Fendlemark sits in front of MLflow, Metaflow, Argo Workflows, and Kubeflow and composes their user experiences into a single, coherent console. Your team stops tab-hopping. Your platform starts to feel like one thing.

[Get started](https://fendlemark.app/docs/getting-started/) [How it works](https://fendlemark.app/docs/architecture/) [Why Fendlemark](https://fendlemark.app/philosophy/)

```shell
$ go install fendlemark.app/fendlemark/cmd/fendlemark@latest
$ fendlemark init my-platform
$ fendlemark serve
fendlemark 0.9.5 · console on http://localhost:7411 · 4 adapters healthy
```

## Four tools. One lattice.

Each adapter reads from a system you already run and contributes *facets*: runs, workflows, artifacts, models, pipelines. Fendlemark correlates them into a single entity graph, the lattice, and the console renders one view per entity, no matter how many tools it came from.

### MLflow

`adapter: mlflow`

Experiments, runs, parameters, metrics, artifacts, and the Model Registry via the tracking server REST API.

### Metaflow

`adapter: metaflow`

Flows, runs, steps, and tasks from the Metaflow metadata service, with datastore artifact links.

### Argo Workflows

`adapter: argo`

Workflows, WorkflowTemplates, and CronWorkflows from the Argo Server API, live status via watch.

### Kubeflow

`adapter: kubeflow`

Pipelines runs and recurring runs from the KFP API, Katib experiments, and Notebook servers by profile.

## Built for platform teams who are tired.

- [A real control plane](https://fendlemark.app/docs/architecture/): A single Go binary. Adapters are compiled in. Reconciliation is deterministic. There is nothing to deploy except the thing itself.
- [A console that is finished](https://fendlemark.app/docs/ui/): Vue 3, composable views, one design language across every tool. Ships with every view your team will need, and none it will not.
- [Adapters you can write](https://fendlemark.app/docs/adapters/writing-an-adapter/): Six methods. One interface. If your platform has a fifth tool, Fendlemark has a fifth adapter the afternoon you decide it does.
- [Respiratory Identity](https://fendlemark.app/docs/auth/): Fendlemark ships its own authentication stack. No identity provider, no passwords, no tokens. Just breathe.

- **4** built-in adapters
- **1** binary to deploy
- **0** loading spinners
- **112** steps to open a pull request

> We strive for perfection and achieve it easily.
>
> — The Fendlemark Philosophy

## Where to next

- [Getting started](https://fendlemark.app/docs/getting-started/): Install the CLI, point it at MLflow and Argo, open the console. Ten minutes.
- [Configuration](https://fendlemark.app/docs/configuration/): Every key in `fendlemark.yaml`, with defaults you will not need to change.
- [Governance](https://fendlemark.app/governance/): How decisions are made, who the Keepers are, and the 112-step Contribution Protocol.
- [FAQ](https://fendlemark.app/faq/): Answers to the questions we are asked, and several we are not.

## Latest from the blog

- [Fendlemark 0.9.5: every merge, explained](https://fendlemark.app/blog/fendlemark-0-9-5/) (24 October 2026 · Release notes): Explain now lists every merge for entities assembled from three or more facets. How to read an explanation, and how to upgrade.
- [Running MLflow and Argo Workflows together](https://fendlemark.app/blog/running-mlflow-and-argo-workflows-together/) (24 October 2026 · How-to): Point Argo steps at MLflow, tag every run with the workflow's identity, hand the run id back to Argo, and find either from the other.
- [Writing a Fendlemark adapter, step by step](https://fendlemark.app/blog/writing-a-fendlemark-adapter/) (24 October 2026 · Walkthrough): Build a working adapter for Kubernetes Jobs, test it with the conformance suite, and compile it into your own Fendlemark binary.

[All posts](https://fendlemark.app/blog/) · [Atom feed](https://fendlemark.app/blog/feed.xml)
