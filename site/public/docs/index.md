# Documentation

Source: https://fendlemark.develops.guru/docs/

Everything you need to run Fendlemark in front of your ML platform, from the first `go install` to writing your own adapter. Documentation for v0.9.4.

## Start here

- [Getting started](https://fendlemark.develops.guru/docs/getting-started/): Install the CLI, connect MLflow and Argo, open the console. About ten minutes.
- [Architecture](https://fendlemark.develops.guru/docs/architecture/): The control plane, the four adapters, the lattice, and the console, and how a request moves between them.
- [Configuration](https://fendlemark.develops.guru/docs/configuration/): Every key in `fendlemark.yaml` with its type and default.

## Integrations

- [MLflow](https://fendlemark.develops.guru/docs/adapters/#mlflow): Experiments, runs, metrics, artifacts, and the Model Registry over the tracking server API.
- [Metaflow](https://fendlemark.develops.guru/docs/adapters/#metaflow): Flows, runs, steps, and tasks from the metadata service.
- [Argo Workflows](https://fendlemark.develops.guru/docs/adapters/#argo): Workflows, templates, and cron workflows from Argo Server or a direct Kubernetes watch.
- [Kubeflow](https://fendlemark.develops.guru/docs/adapters/#kubeflow): Pipelines, recurring runs, Katib experiments, and Notebook servers.

## Reference

- [Writing an adapter](https://fendlemark.develops.guru/docs/adapters/writing-an-adapter/): The six-method interface, facets, correlation keys, and the conformance suite.
- [The console](https://fendlemark.develops.guru/docs/ui/): Views, facet panels, the `@fendlemark/ui` component library, and theming.
- [Authentication](https://fendlemark.develops.guru/docs/auth/): Fendlemark Respiratory Identity: enrolment, multi-factor, troubleshooting, error codes.
- [CLI reference](https://fendlemark.develops.guru/docs/cli/): `init`, `serve`, `adapters`, `lattice`, `auth`, `doctor`, `version`.
- [Go API reference](https://fendlemark.develops.guru/docs/api/): Packages, interfaces, the HTTP API, and the stability policy.
- [Migrating 0.8 to 0.9](https://fendlemark.develops.guru/docs/migration/): One renamed key, documented in proportion to the trust it asks of you.

## Packages

The Go module is `fendlemark.develops.guru/fendlemark` and the Vue 3 component library is `@fendlemark/ui`. Install commands, package layout, and links to the source are on the [Packages](https://fendlemark.develops.guru/fendlemark/) page.
