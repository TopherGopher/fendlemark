# Adapters

Source: https://fendlemark.develops.guru/docs/adapters/

An adapter connects Fendlemark to one system you already run and translates what it finds into facets. Four ship in the binary. All of them are read-only. This page says exactly what each one reads, surfaces, correlates on, and leaves alone.

## Common behaviour

- **Read-only.** No adapter creates, updates, deletes, triggers, retries, or cancels anything upstream. The console deep-links to the tool for those actions.
- **Least privilege.** Each adapter documents the minimum permission it needs. Grant that and nothing more.
- **Graceful degradation.** An unreachable upstream marks the adapter `degraded`, keeps its last facets for `lattice.retain_orphans`, and keeps the console up. The other adapters are unaffected.
- **Health.** `fendlemark adapters check` and `GET /api/v1/adapters` report status, latency, and a one-line detail per adapter.
- **Maturity.** Every adapter has a maturity level on the ladder defined in [Governance](https://fendlemark.develops.guru/governance/#maturity): Provisional, Settled, Established, Canonical. All four built-in adapters are Canonical. No adapter has ever been demoted.

## mlflow

**Reads from:** an MLflow tracking server over its REST API (`/api/2.0/mlflow/experiments/search`, `/runs/search`, `/metrics/get-history`, `/artifacts/list`, and the `registered-models` and `model-versions` endpoints when `registry: true`). Works with any tracking server backend, since Fendlemark only sees the API. Also understands `MLFLOW_TRACKING_USERNAME` and `MLFLOW_TRACKING_PASSWORD` for basic auth, and `MLFLOW_TRACKING_TOKEN` for bearer auth.

**Facets contributed:** Experiment, Run (with params, tags, latest metric values, lifecycle stage), Metric series (fetched on demand when the Run view is open), Artifact listing (names and sizes; content is deep-linked, never proxied), Registered Model, Model Version (with stage or alias, source run, and signature summary).

**Correlation keys provided:** `mlflow.run_id` (always), `mlflow.experiment_id`, and whatever it finds in the tags listed in `correlation_tags`. The default list is:

```yaml
correlation_tags:
  - argo.workflow.name
  - fendlemark.argo_workflow
  - metaflow.run_id
  - metaflow.flow_name
  - kfp.run_id
  - mlflow.source.git.commit   # mapped to git.sha
  - mlflow.source.git.repoURL  # mapped to git.repo
```

MLflow sets the two `mlflow.source.git.*` tags automatically when a run is started from a git checkout, so the weak `git.sha` key usually works without any effort on your part.

**Does not:** log metrics, create runs, transition model stages, or proxy artifact bytes. Does not read the tracking server's own UI; everything comes from the API.

**Minimum permission:** read access to the experiments you list (or all, if you list none). On a tracking server with the basic-auth plugin, a user with `READ` on the relevant experiments and registered models is sufficient.

## metaflow

**Reads from:** the Metaflow metadata service, the same service the Metaflow client and the Metaflow UI backend read. Fendlemark does not read the datastore itself; `datastore_root` is used only to construct links to artifacts (for example an S3 console URL for a step's artifact prefix).

**Facets contributed:** Flow, Run (with user, tags, system tags, start and end), Step, Task (with attempt count and status), Artifact listing (names only), and the Metaflow-side view of any `@kubernetes` or `@batch` step's compute metadata where the metadata service records it.

**Correlation keys provided:** `metaflow.run_id` (in Metaflow's `FlowName/RunId` form and the bare numeric or Argo-generated form, since both appear in the wild), `metaflow.flow_name`, `argo.workflow.name` when the run was launched by a Metaflow-generated Argo Workflow (Metaflow records this in system tags), and `git.sha` when the `@project` decorator or a user tag carries it.

**Does not:** resume, trigger, or tag runs. Does not read artifact contents; Metaflow artifacts can be large and Fendlemark has no business holding them.

**Minimum permission:** network access to the metadata service. The metadata service itself has no built-in auth; if you front it with a proxy, give Fendlemark a read-only identity there.

## argo

**Reads from:** Argo Server's REST API (`/api/v1/workflows/{namespace}`, `/workflow-templates`, `/cron-workflows`, and `/archived-workflows` when `include_archived: true` and the server has an archive configured). With `kube_watch: true`, it instead watches `Workflow` resources directly through the Kubernetes API for lower-latency updates, and uses Argo Server only for the archive and templates.

**Facets contributed:** Workflow (phase, start and finish, labels, annotations, parameters, outputs), Node (each step or DAG task with phase, template name, pod name, and a link to logs), WorkflowTemplate, CronWorkflow (schedule, last scheduled time, active count).

**Correlation keys provided:** `argo.workflow.name` (always), `argo.workflow.uid`, `kfp.run_id` from the `pipeline/runid` label that Kubeflow Pipelines sets on Workflows it creates, `metaflow.run_id` from Metaflow's Argo-generated labels and annotations, and `git.sha` from the `fendlemark.io/git-sha` annotation or the common `app.kubernetes.io/version` label when it looks like a commit.

**Does not:** submit, resubmit, retry, stop, terminate, suspend, or delete workflows. Does not stream logs through Fendlemark; the Node facet links to Argo's own log viewer.

**Minimum permission:** an Argo Server token (or kubeconfig context) with `get`, `list`, and `watch` on `workflows`, `workflowtemplates`, and `cronworkflows` in the listed namespaces. Nothing cluster-wide.

## kubeflow

**Reads from:** the Kubeflow Pipelines API (pipelines, pipeline versions, runs, recurring runs, and experiments, scoped by profile when you list them). With `katib: true` it reads Katib `Experiment` and `Trial` resources through the Kubernetes API, and with `notebooks: true` it reads `Notebook` resources the same way.

**Facets contributed:** Pipeline (with versions), Pipeline Run (status, parameters, created and finished), Recurring Run (trigger, enabled, next scheduled), KFP Experiment, Katib Experiment (objective, algorithm, best trial summary), Notebook Server (image, resources, status).

**Correlation keys provided:** `kfp.run_id` (always), `kfp.pipeline_id`, `argo.workflow.name` because a KFP run's underlying Argo Workflow name is recoverable from the run detail, and `metaflow.run_id` or `mlflow.run_id` when they appear as pipeline run parameters named exactly that.

Because Kubeflow Pipelines executes on Argo Workflows by default, enabling both `argo` and `kubeflow` for the same cluster produces one entity per pipeline run with both facets attached, which is the single most common reason teams adopt Fendlemark.

**Does not:** create or terminate runs, enable or disable recurring runs, start or stop notebooks, or read notebook contents.

**Minimum permission:** a service account with read access to the KFP API for the listed profiles, plus `get`, `list`, `watch` on `experiments.kubeflow.org`, `trials.kubeflow.org`, and `notebooks.kubeflow.org` in those namespaces if Katib and Notebooks are enabled.

## Correlation matrix

Which keys connect which adapters, out of the box, with no configuration beyond enabling them:

|  | mlflow | metaflow | argo | kubeflow |
| --- | --- | --- | --- | --- |
| mlflow | — | `metaflow.run_id` (tag) | `argo.workflow.name` (tag) | `kfp.run_id` (tag) |
| metaflow | `metaflow.run_id` | — | `argo.workflow.name` (system tag) | `metaflow.run_id` (parameter) |
| argo | `argo.workflow.name` | `metaflow.run_id` (label) | — | `kfp.run_id` (label, automatic) |
| kubeflow | `mlflow.run_id` (parameter) | `metaflow.run_id` (parameter) | `argo.workflow.name` (automatic) | — |

"Automatic" means the upstream tools already record the key without your pipelines doing anything. Everything else requires that your code set a tag, label, or parameter, which most teams find they were already doing for their own sanity.

## A fifth tool?

Write an adapter. The interface has six methods and is documented in [Writing an adapter](https://fendlemark.develops.guru/docs/adapters/writing-an-adapter/). Proposing it for inclusion in the binary is governed by [FCP-1](https://fendlemark.develops.guru/governance/contributing/), which the Keepers describe as "thorough".
