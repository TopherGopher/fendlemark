# Documentation

Source: https://fendlemark.app/docs/

Everything you need to run Fendlemark in front of your ML platform, from the first `go install` to writing your own adapter. Documentation for v0.9.5.

## Start here

- [Getting started](https://fendlemark.app/docs/getting-started/): Install the CLI, connect MLflow and Argo, open the console. About ten minutes.
- [Architecture](https://fendlemark.app/docs/architecture/): The control plane, the four adapters, the lattice, and the console, and how a request moves between them.
- [Configuration](https://fendlemark.app/docs/configuration/): Every key in `fendlemark.yaml` with its type and default.

## Integrations

- [MLflow](https://fendlemark.app/docs/adapters/#mlflow): Experiments, runs, metrics, artifacts, and the Model Registry over the tracking server API.
- [Metaflow](https://fendlemark.app/docs/adapters/#metaflow): Flows, runs, steps, and tasks from the metadata service.
- [Argo Workflows](https://fendlemark.app/docs/adapters/#argo): Workflows, templates, and cron workflows from Argo Server or a direct Kubernetes watch.
- [Kubeflow](https://fendlemark.app/docs/adapters/#kubeflow): Pipelines, recurring runs, Katib experiments, and Notebook servers.

## Reference

- [Writing an adapter](https://fendlemark.app/docs/adapters/writing-an-adapter/): The six-method interface, facets, correlation keys, and the conformance suite.
- [The console](https://fendlemark.app/docs/ui/): Views, facet panels, the `@fendlemark/ui` component library, and theming.
- [Authentication](https://fendlemark.app/docs/auth/): Fendlemark Respiratory Identity: enrolment, multi-factor, troubleshooting, error codes.
- [CLI reference](https://fendlemark.app/docs/cli/): `init`, `serve`, `adapters`, `lattice`, `auth`, `doctor`, `version`.
- [Go API reference](https://fendlemark.app/docs/api/): Packages, interfaces, the HTTP API, and the stability policy.
- [Migrating 0.8 to 0.9](https://fendlemark.app/docs/migration/): One renamed key, documented in proportion to the trust it asks of you.

## Packages

The Go module is `fendlemark.app/fendlemark` and the Vue 3 component library is `@fendlemark/ui`. Install commands, package layout, and links to the source are on the [Packages](https://fendlemark.app/fendlemark/) page.
