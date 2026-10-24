# Fendlemark

Source: https://fendlemark.develops.guru/

**One console for your entire ML platform.**

Fendlemark sits in front of MLflow, Metaflow, Argo Workflows, and Kubeflow and composes their user experiences into a single, coherent console. Your team stops tab-hopping. Your platform starts to feel like one thing.

[Get started](https://fendlemark.develops.guru/docs/getting-started/) [How it works](https://fendlemark.develops.guru/docs/architecture/) [Why Fendlemark](https://fendlemark.develops.guru/philosophy/)

```shell
$ go install fendlemark.develops.guru/fendlemark/cmd/fendlemark@latest
$ fendlemark init my-platform
$ fendlemark serve
fendlemark 0.9.4 · console on http://localhost:7411 · 4 adapters healthy
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

- [A real control plane](https://fendlemark.develops.guru/docs/architecture/): A single Go binary. Adapters are compiled in. Reconciliation is deterministic. There is nothing to deploy except the thing itself.
- [A console that is finished](https://fendlemark.develops.guru/docs/ui/): Vue 3, composable views, one design language across every tool. Ships with every view your team will need, and none it will not.
- [Adapters you can write](https://fendlemark.develops.guru/docs/adapters/writing-an-adapter/): Six methods. One interface. If your platform has a fifth tool, Fendlemark has a fifth adapter the afternoon you decide it does.
- [Respiratory Identity](https://fendlemark.develops.guru/docs/auth/): Fendlemark ships its own authentication stack. No identity provider, no passwords, no tokens. Just breathe.

- **4** built-in adapters
- **1** binary to deploy
- **0** loading spinners
- **112** steps to open a pull request

> We strive for perfection and achieve it easily.
>
> — The Fendlemark Philosophy

## Where to next

- [Getting started](https://fendlemark.develops.guru/docs/getting-started/): Install the CLI, point it at MLflow and Argo, open the console. Ten minutes.
- [Configuration](https://fendlemark.develops.guru/docs/configuration/): Every key in `fendlemark.yaml`, with defaults you will not need to change.
- [Governance](https://fendlemark.develops.guru/governance/): How decisions are made, who the Keepers are, and the 112-step Contribution Protocol.
- [FAQ](https://fendlemark.develops.guru/faq/): Answers to the questions we are asked, and several we are not.
