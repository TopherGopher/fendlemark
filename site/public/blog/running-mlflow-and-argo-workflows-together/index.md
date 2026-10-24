# Running MLflow and Argo Workflows together

Source: https://fendlemark.app/blog/running-mlflow-and-argo-workflows-together/

By [the Fendlemark Keepers](https://fendlemark.app/governance/#keepers) · 24 October 2026

Argo Workflows is a good place to run training, and MLflow is a good place to record it. Used together without any care, they give every training job two identities in two UIs, and nothing that connects them. This guide makes each side point at the other. None of it needs Fendlemark; the last section explains what Fendlemark does with it if you run it.

By the end, every MLflow run started by a workflow step will carry the workflow's name, uid, namespace, and pod as tags, every workflow will carry the MLflow run id as an output parameter, and a later step will log into the same MLflow run rather than starting another. The examples use a Workflow named `resnet50-train-…` in the `ml-training` namespace and an MLflow tracking server reachable inside the cluster at `http://mlflow.mlflow.svc:5000`. Substitute your own.

## 1. Point every step at the tracking server

The MLflow client reads its tracking server from the `MLFLOW_TRACKING_URI` environment variable, so a step needs no MLflow configuration in code. If your tracking server uses MLflow's basic authentication, the client also reads `MLFLOW_TRACKING_USERNAME` and `MLFLOW_TRACKING_PASSWORD`; a bearer token goes in `MLFLOW_TRACKING_TOKEN` instead.

Keep the address in a ConfigMap and the credentials in a Secret, with keys named exactly like the variables, so each step can import both with `envFrom`:

```shell
$ kubectl -n ml-training create configmap mlflow-client --from-literal=MLFLOW_TRACKING_URI=http://mlflow.mlflow.svc:5000
$ kubectl -n ml-training create secret generic mlflow-credentials --from-literal=MLFLOW_TRACKING_USERNAME=argo-runner --from-literal=MLFLOW_TRACKING_PASSWORD="$MLFLOW_PASSWORD"
```

Changing the tracking server later is then one ConfigMap edit, not an edit to every template.

## 2. Give each step the workflow's identity

Argo substitutes template variables in a template's fields before the pod starts, including environment variable values. Four of them identify where a step is running:

| Variable | Value |
| --- | --- |
| `{{workflow.name}}` | The Workflow's name, for example `resnet50-train-x7k2p` when it was submitted with `generateName: resnet50-train-`. Unique within its namespace while the Workflow exists. |
| `{{workflow.uid}}` | The Workflow's Kubernetes UID. Unique across namespaces and across time, so it still identifies this run after a Workflow with the same name has been deleted and submitted again. |
| `{{workflow.namespace}}` | The namespace. With the name, enough to find the Workflow in the Argo UI or the CLI. |
| `{{pod.name}}` | The step's pod, available in container and script templates. Tells you which node of the workflow logged the run. |

Here is a two-step Workflow that trains and then evaluates. The training step gets the tracking server from the ConfigMap and Secret, its identity from template variables, and declares an output parameter read from a file the script will write:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: resnet50-train-
  namespace: ml-training
spec:
  entrypoint: main
  arguments:
    parameters:
      - name: lr
        value: "0.0003"
  templates:
    - name: main
      steps:
        - - name: train
            template: train
        - - name: evaluate
            template: evaluate
            arguments:
              parameters:
                - name: mlflow-run-id
                  value: "{{steps.train.outputs.parameters.mlflow-run-id}}"

    - name: train
      outputs:
        parameters:
          - name: mlflow-run-id
            valueFrom:
              path: /tmp/mlflow-run-id
      container:
        image: registry.example.internal/vision/train:2026.10
        command: [python, train.py]
        args: ["--lr", "{{workflow.parameters.lr}}"]
        envFrom:
          - configMapRef:
              name: mlflow-client
          - secretRef:
              name: mlflow-credentials
        env:
          - name: ARGO_WORKFLOW_NAME
            value: "{{workflow.name}}"
          - name: ARGO_WORKFLOW_UID
            value: "{{workflow.uid}}"
          - name: ARGO_WORKFLOW_NAMESPACE
            value: "{{workflow.namespace}}"
          - name: ARGO_POD_NAME
            value: "{{pod.name}}"

    - name: evaluate
      inputs:
        parameters:
          - name: mlflow-run-id
      container:
        image: registry.example.internal/vision/train:2026.10
        command: [python, evaluate.py]
        envFrom:
          - configMapRef:
              name: mlflow-client
          - secretRef:
              name: mlflow-credentials
        env:
          - name: MLFLOW_RUN_ID
            value: "{{inputs.parameters.mlflow-run-id}}"
```

The quotes around each `{{…}}` value matter: unquoted, YAML would read the braces as a mapping.

## 3. Tag the MLflow run with that identity

MLflow tags are free-form string pairs on a run, searchable and shown in the run's page. `mlflow.start_run` accepts them directly, so the run carries the workflow's identity from the moment it exists:

```python
import argparse
import os

import mlflow


def argo_tags():
    """Where this run came from, as MLflow tags."""
    return {
        "argo.workflow.name": os.environ["ARGO_WORKFLOW_NAME"],
        "argo.workflow.uid": os.environ["ARGO_WORKFLOW_UID"],
        "argo.workflow.namespace": os.environ["ARGO_WORKFLOW_NAMESPACE"],
        "argo.pod.name": os.environ["ARGO_POD_NAME"],
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--lr", type=float, required=True)
    args = parser.parse_args()

    mlflow.set_experiment("resnet50")
    with mlflow.start_run(run_name=os.environ["ARGO_WORKFLOW_NAME"], tags=argo_tags()) as run:
        # Hand the run id back to Argo: this file is the step's output parameter.
        with open("/tmp/mlflow-run-id", "w") as f:
            f.write(run.info.run_id)

        mlflow.log_param("lr", args.lr)
        for epoch, metrics in enumerate(train(args.lr)):  # train() is your own loop
            mlflow.log_metrics(metrics, step=epoch)


if __name__ == "__main__":
    main()
```

A few choices in there are deliberate:

- **The run is named after the Workflow.** MLflow does not require run names to be unique, and a name that matches the Argo UI makes the two easy to match by eye.
- **The run id is written first.** Writing it before training means the file exists however long training takes, and the Argo UI shows it in the step's outputs once the step finishes.
- **The tags use the names `argo.workflow.name` and `argo.workflow.uid`.** Any names would do for MLflow. These are the ones Fendlemark reads, as the last section explains.

Container images rarely include a `.git` directory, so MLflow usually cannot detect the commit it would otherwise record as `mlflow.source.git.commit`. If you bake the commit into the image at build time, set that tag yourself, along with `mlflow.source.git.repoURL`, in the same dictionary.

## 4. Log later steps into the same run

The `evaluate` step receives the training run's id as an input parameter and exposes it as `MLFLOW_RUN_ID`. When that variable is set, `mlflow.start_run()` resumes the run with that id instead of creating a new one, so evaluation metrics land next to the training metrics:

```python
import mlflow

# MLFLOW_RUN_ID is set by the workflow, so this resumes the training run.
with mlflow.start_run() as run:
    mlflow.log_metrics(evaluate(run.info.run_id))  # evaluate() is your own code
```

One MLflow run per workflow is easier to compare across workflows than one per step, and it keeps the correlation simple: one workflow, one run. Steps that genuinely produce separate results, such as a hyperparameter sweep fanned out with `withItems`, are better as separate runs; pass the parent's id as `parent_run_id` to `mlflow.start_run` to nest them under it.

## 5. Find one from the other

**From a workflow to its MLflow run.** The `train` node's outputs in the Argo UI show `mlflow-run-id`. To go the other way round without the UI, look the run up by the workflow's uid, which is unambiguous even when names repeat:

```shell
$ kubectl -n ml-training get workflow resnet50-train-x7k2p -o jsonpath='{.metadata.uid}'
51d0c8e2-7b3a-4e19-a6f4-0c9d2e8b7a13
```

```python
import mlflow

runs = mlflow.search_runs(
    filter_string="tags.`argo.workflow.uid` = '51d0c8e2-7b3a-4e19-a6f4-0c9d2e8b7a13'",
    search_all_experiments=True,
)
print(runs[["run_id", "status", "tags.argo.pod.name"]])
```

Tag names that contain dots must be wrapped in backticks (or double quotes) in an MLflow filter, and the value in single quotes. The same filter works in the search box of the MLflow UI: `` tags.`argo.workflow.name` = 'resnet50-train-x7k2p' ``.

**From an MLflow run to its workflow.** The run's `argo.workflow.namespace` and `argo.workflow.name` tags are everything the Argo CLI needs:

```shell
$ argo get resnet50-train-x7k2p -n ml-training
```

In the Argo UI, the same Workflow is at `/workflows/ml-training/resnet50-train-x7k2p` on your Argo Server.

## Things that will happen eventually

- **Names repeat.** A Workflow submitted with a fixed `metadata.name` instead of `generateName` gets the same name every time it is deleted and resubmitted, and the same name can exist in two namespaces at once. Search by `argo.workflow.uid` when it matters which one you mean.
- **Retries make more runs.** If the training template has a `retryStrategy`, every attempt is a new pod, and with the script above a new MLflow run with a different `argo.pod.name`. Templates with a `retryStrategy` can also use `{{retries}}`, the attempt number, which is worth adding as a tag of its own.
- **Killed pods leave running runs.** The `with mlflow.start_run()` block marks a run `FAILED` when Python raises, but a pod killed for exceeding its memory limit never reaches the end of the block, and its run stays `RUNNING`. Search for `attributes.status = 'RUNNING'` runs whose workflow has finished to find them.
- **The tracking server is a dependency.** If MLflow is down, `start_run` fails once the client has exhausted its retries, and so does the step. That is usually what you want for training; if it is not, catch the error and decide explicitly.

## How Fendlemark correlates them

If you run Fendlemark with the [`mlflow`](https://fendlemark.app/docs/adapters/#mlflow) and [`argo`](https://fendlemark.app/docs/adapters/#argo) adapters enabled, the tags above are all it needs. The argo adapter gives every Workflow facet the correlation key `argo.workflow.name`, and `argo.workflow.uid` alongside it. The mlflow adapter reads the run tags listed in `correlation_tags`, whose default list includes `argo.workflow.name` (and `fendlemark.argo_workflow`, which it treats the same way). When a Workflow facet and an MLflow run facet carry the same value, and their start times are no further apart than `lattice.correlation.window`, the lattice merges them into one entity:

```shell
$ fendlemark lattice inspect --key argo.workflow.name=resnet50-train-x7k2p
ID                   KIND  STATUS     FACETS       STARTED               TITLE
lt_459baa2c413ad692  run   Succeeded  argo+mlflow  2026-10-22T09:00:00Z  resnet50-train-x7k2p
$ fendlemark lattice explain lt_459baa2c413ad692
lt_459baa2c413ad692  run  Succeeded  "resnet50-train-x7k2p"
  facet  mlflow     6f1d2c9a8b7e4f30a1c5d9e8b2a47c13
  facet  argo       ml-training/resnet50-train-x7k2p
  merge  argo.workflow.name = "resnet50-train-x7k2p" linked argo/ml-training/resnet50-train-x7k2p and mlflow/6f1d2c9a8b7e4f30a1c5d9e8b2a47c13 (gap 14s)
```

Three details of the rules are worth knowing when you design your workflows:

- **The window is measured between start times:** the Workflow's start and the MLflow run's start. The default, 90 seconds, suits a workflow whose tracked step comes first. If the step that opens the MLflow run follows a long data-preparation step, widen it to cover that step:

  ```yaml
  lattice:
    correlation:
      window: 20m
  ```
- **Two facets from the same adapter never merge.** A workflow is merged with one MLflow run: the first to start after it. That is why step 4 resumes the training run instead of starting a second one. Extra runs carrying the same tag, such as retried attempts, stay entities of their own, and `fendlemark lattice inspect --key argo.workflow.name=…` lists all of them.
- **To correlate on the uid instead of the name,** list it in `correlation_tags`. The list replaces the default rather than adding to it, so name every tag you want read:

  ```yaml
  adapters:
    mlflow:
      correlation_tags:
        - argo.workflow.uid
        - mlflow.source.git.commit
        - mlflow.source.git.repoURL
  ```

  The argo adapter already provides `argo.workflow.uid` for every Workflow, so nothing changes on that side. Do this if the same Workflow name can start in two namespaces within the window.

The weak keys are a separate mechanism. If the MLflow run carries `mlflow.source.git.commit` and `mlflow.source.git.repoURL`, and the Workflow carries the same values in the `fendlemark.io/git-sha` and `fendlemark.io/git-repo` annotations, Fendlemark relates entities built from the same commit without merging them. The [architecture page](https://fendlemark.app/docs/architecture/#correlation) describes both kinds of key, and the [correlation matrix](https://fendlemark.app/docs/adapters/#matrix) shows which keys connect which adapters.

## Further reading

- [Adapters: mlflow](https://fendlemark.app/docs/adapters/#mlflow) and [argo](https://fendlemark.app/docs/adapters/#argo), including the full `correlation_tags` default list
- [Configuration: `lattice.correlation.window`](https://fendlemark.app/docs/configuration/#lattice)
- [CLI reference: `lattice inspect` and `lattice explain`](https://fendlemark.app/docs/cli/#lattice)
- [Getting started](https://fendlemark.app/docs/getting-started/), which connects exactly these two tools
