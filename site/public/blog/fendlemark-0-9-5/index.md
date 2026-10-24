# Fendlemark 0.9.5: every merge, explained

Source: https://fendlemark.app/blog/fendlemark-0-9-5/

By [the Fendlemark Keepers](https://fendlemark.app/governance/#keepers) · 24 October 2026

Fendlemark 0.9.5 is a Clarification. When an entity is assembled from three or more facets, `fendlemark lattice explain` and the explain endpoint of the API now list every merge that assembled it. The lattice itself has not changed: the same facets reconcile into the same entities, with the same ids, as they did before.

## What Explain is for

The [lattice](https://fendlemark.app/docs/architecture/#lattice) takes the facets every adapter reports and decides which of them describe the same thing. It decides by exact equality on [correlation keys](https://fendlemark.app/docs/architecture/#correlation), within the correlation window, and never by resemblance. That makes every entity accountable: it can say which facets it holds, which key joined each pair, and how far apart the two facets started. `fendlemark lattice explain` prints that account, and `GET /api/v1/entities/{id}/explain` returns it as JSON.

The [CLI reference](https://fendlemark.app/docs/cli/#lattice) describes Explain as the single most useful command for "why are these two things one thing". The Keepers see no reason to revise that. They have only made sure that, from this release, the answer is always complete.

## Reading an explanation

Take a nightly training pipeline in Kubeflow Pipelines. KFP executes it on Argo Workflows, and its training step logs to MLflow, tagging the MLflow run with the pipeline run's id as `kfp.run_id`. With the `kubeflow`, `argo`, and `mlflow` adapters enabled, that one job is three facets: the pipeline run, the Workflow it ran as, and the MLflow run. `lattice inspect` shows them as one entity:

```shell
$ fendlemark lattice inspect --kind run
ID                   KIND  STATUS     FACETS                STARTED               TITLE
lt_105a29352cf19efd  run   Succeeded  argo+kubeflow+mlflow  2026-10-22T02:00:00Z  resnet50-nightly-train
```

Entity ids are derived from the set of facets that merged, so the id `inspect` prints is the id `explain` accepts, on any machine whose adapters see the same facets:

```shell
$ fendlemark lattice explain lt_105a29352cf19efd
lt_105a29352cf19efd  run  Succeeded  "resnet50-nightly-train"
  facet  mlflow     6f1d2c9a8b7e4f30a1c5d9e8b2a47c13
  facet  argo       kubeflow/resnet50-nightly-8wq5t
  facet  kubeflow   run:9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83
  merge  kfp.run_id = "9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83" linked kubeflow/run:9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83 and argo/kubeflow/resnet50-nightly-8wq5t (gap 2s)
  merge  kfp.run_id = "9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83" linked argo/kubeflow/resnet50-nightly-8wq5t and mlflow/6f1d2c9a8b7e4f30a1c5d9e8b2a47c13 (gap 39s)
```

Line by line:

- **The first line** is the entity: id, kind, status, and title. The kind is `run` because a run outranks a workflow when facets of different kinds merge (the order is run, workflow, pipeline, model, artifact, notebook). The title is the MLflow run's, because facets are listed in adapter order, mlflow, metaflow, argo, kubeflow, and the entity takes its title from the first.
- **Each `facet` line** is one adapter's view of the job: the adapter and the identifier it uses upstream. The argo adapter identifies a Workflow by namespace and name; the kubeflow adapter prefixes run ids with `run:`.
- **Each `merge` line** is one decision: the key and value two facets shared, the two facets, and the gap between their start times. The gap is what the reconciler compared against `lattice.correlation.window`.
- **`edge` lines**, when present, are relations rather than merges: a model version `DerivedFrom` the run that produced it, or two entities that `SharesCommit` through the weak `git.sha` and `git.repo` keys. This entity has none.

The merges read as a chain, and the chain has an order. For each key value, the reconciler sorts the facets that carry it by start time and links each one to its neighbour. So the explanation also tells you how the job began: KFP scheduled the run, Argo started the Workflow two seconds later, and the training step opened its MLflow run thirty-nine seconds after that. The pipeline run and the MLflow run were never compared with each other directly, and did not need to be.

## What 0.9.5 clarifies

An entity assembled from *n* facets has exactly *n* − 1 merges. Each merge joins two groups of facets that were apart, and it takes *n* − 1 joins to make one group of *n*. Two facets, one merge; three facets, two merges, as above.

Before 0.9.5, an explanation could come up short of that number. The reconciler filed each merge under whichever facet was the entity's root when the merge was made, and a later merge could move the root to a different facet. The merges filed under the earlier root were then left out of the explanation. The entity was always assembled correctly, with the right facets and the right id; only its account of itself was incomplete, and only when it had three or more facets. 0.9.5 records the merges in the order they are made and files them once the entity is complete.

You can check the arithmetic for any entity. Facets minus merges is always one:

```shell
$ fendlemark lattice explain --json lt_105a29352cf19efd | jq '(.entity.facets | length) - (.merges | length)'
1
```

## The JSON

`fendlemark lattice explain --json` and `GET /api/v1/entities/{id}/explain` return the same object: `entity` (the entity, with every facet in full), `merges`, and `relations`. The merges for the example:

```shell
$ fendlemark lattice explain --json lt_105a29352cf19efd | jq '.merges'
[
  {
    "a": "kubeflow\u0000run:9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83",
    "b": "argo\u0000kubeflow/resnet50-nightly-8wq5t",
    "key": "kfp.run_id",
    "value": "9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83",
    "gap": 2000000000
  },
  {
    "a": "argo\u0000kubeflow/resnet50-nightly-8wq5t",
    "b": "mlflow\u00006f1d2c9a8b7e4f30a1c5d9e8b2a47c13",
    "key": "kfp.run_id",
    "value": "9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83",
    "gap": 39000000000
  }
]
```

Three details are worth knowing before you script against it:

- `a` and `b` are facet references: the adapter name, a NUL character, then the facet's source id. JSON writes the NUL as `\u0000`. Split on it rather than on `/`, which source ids are free to contain.
- `gap` is in nanoseconds. Divide by `1e9` for seconds.
- `relations` is `null`, not `[]`, when an entity has no edges. Treat the two the same.

The same request against a running console, printed one merge per line:

```shell
$ curl -s http://127.0.0.1:7411/api/v1/entities/lt_105a29352cf19efd/explain | jq -r '.merges[] | "\(.key)  \(.a | split("\u0000") | join("/")) + \(.b | split("\u0000") | join("/"))  \(.gap / 1e9)s"'
kfp.run_id  kubeflow/run:9c1e4b72-3a5d-4f08-b6e1-2d7a9c0f5e83 + argo/kubeflow/resnet50-nightly-8wq5t  2s
kfp.run_id  argo/kubeflow/resnet50-nightly-8wq5t + mlflow/6f1d2c9a8b7e4f30a1c5d9e8b2a47c13  39s
```

The full schema is `Explanation` in [openapi.json](https://fendlemark.app/openapi.json).

## Why the list matters

An entity is a claim that several records in several tools are one job. The merge list is the evidence for that claim, and evidence with a page missing is not evidence. In practice, the complete list is useful in three ways.

- **It shows which key did the work.** In the example, both merges came from `kfp.run_id`: one from the `pipeline/runid` label that Kubeflow Pipelines puts on the Workflows it creates, one from a tag the training step set. If the tag disappears in a refactor, the MLflow run will stand alone again, and the explanation tells you in advance which line of whose code it depends on.
- **It shows how close each merge came to not happening.** The window is measured between start times. A merge with a gap near `lattice.correlation.window` (90 seconds by default) is one that a slower node or a longer queue would have prevented. To list the merges with a gap over thirty seconds:

  ```shell
  $ fendlemark lattice explain --json lt_105a29352cf19efd | jq -c '.merges[] | select(.gap > 30e9) | {key, gap_s: (.gap / 1e9)}'
  {"key":"kfp.run_id","gap_s":39}
  ```
- **It shows the path.** When an entity holds a facet you did not expect, the merges say which neighbour brought it in, and on which key. This is the question an incomplete explanation answered worst: with a merge missing, a facet could sit in an entity with no line saying how it got there.

## Upgrading

There are no configuration changes in 0.9.5, and `fendlemark doctor` has nothing new to say. If you still spell `adapters.mlflow.trackingUri` the pre-0.9 way, it is accepted with `DOC-104` as before; see the [migration guide](https://fendlemark.app/docs/migration/).

### The CLI

```shell
$ go install fendlemark.app/fendlemark/cmd/fendlemark@v0.9.5
$ fendlemark version | head -n 1
fendlemark 0.9.5 (go1.23.2, linux/amd64)
```

`@latest` resolves to the same release. Go 1.23 or later is required. Restart `fendlemark serve` to pick up the new binary; the lattice rebuilds from the adapters on start, so there is nothing to migrate.

### Embedding the Go module

```shell
$ go get fendlemark.app/fendlemark@v0.9.5
$ go mod tidy
```

`fendlemark.Version` is now `"0.9.5"`. The `adapter` package is unchanged, as [FDR-027](https://fendlemark.app/governance/#decisions) requires. `lattice.Explanation` has the same fields as before; `Merges` is now always complete. If your own code reconstructed missing merges from facet keys, it can stop.

Adapters identify themselves upstream with a `User-Agent` of `fendlemark/0.9.5`. If a proxy in front of MLflow, Argo Server, or the KFP API matches on the version, update the rule.

### The console components

```shell
$ npm install @fendlemark/ui@0.9.5
```

`@fendlemark/ui` 0.9.5 has no component or prop changes. Its `VERSION` export reads `'0.9.5'`, in lockstep with the Go module, as it always does; see [Packages](https://fendlemark.app/fendlemark/#versions).

## How it was released

0.9.5 followed [release governance](https://fendlemark.app/governance/#releases) in full: a Release Intent classifying the change as a Clarification, a Reflection Period, four independent and byte-identical builds, unanimous consent, and a Silence Period. The changelog entry was approved with the change and appears in the [changelog](https://fendlemark.app/changelog/#0-9-5) unmodified.

## Further reading

- [CLI reference: `fendlemark lattice`](https://fendlemark.app/docs/cli/#lattice)
- [Architecture: correlation keys and reconciliation](https://fendlemark.app/docs/architecture/#correlation)
- [Go API reference: package lattice](https://fendlemark.app/docs/api/#lattice-pkg)
- [Configuration: `lattice.correlation.window`](https://fendlemark.app/docs/configuration/#lattice)
