# Configuration

Source: https://fendlemark.develops.guru/docs/configuration/

Fendlemark reads one file, `fendlemark.yaml`, from the working directory or the path given by `--config`. Every key has a default. Most installations change five of them.

## Precedence

Values are resolved in this order, highest priority first:

1. Command-line flags (`--listen`, `--config`, `--log-level`).
2. Environment variables prefixed `FENDLEMARK_`, with nesting expressed by double underscores: `FENDLEMARK_ADAPTERS__MLFLOW__TRACKING_URI`.
3. `fendlemark.yaml`.
4. Built-in defaults.

Any string value may reference an environment variable with `${VAR}` syntax. Secrets should be supplied this way rather than written into the file.

## A minimal file

```yaml
adapters:
  mlflow:
    enabled: true
    tracking_uri: "http://mlflow.internal:5000"
```

That is a complete, valid configuration. Everything below is optional.

## server

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `server.listen` | string | `"127.0.0.1:7411"` | Address the console and API listen on. The default accepts connections from this machine only; to share the console, keep it on loopback behind an authenticating reverse proxy rather than widening it (doctor reports `DOC-121` if you do). 7411 was chosen because it was free on the first Keeper's laptop in 2022 and has not been reconsidered. |
| `server.base_url` | string | derived | Public URL of the console, used in links. Set it when running behind a reverse proxy. |
| `server.data_dir` | string | `"./.fendlemark"` | Where persistent state (preferences, FRI enrolments) is kept. |
| `server.read_timeout` | duration | `15s` | HTTP read timeout. |
| `server.shutdown_grace` | duration | `10s` | Time to let in-flight requests finish on SIGTERM. |
| `server.tls.cert_file` | string | unset | Serve TLS directly. Most deployments terminate TLS at a proxy instead. |
| `server.tls.key_file` | string | unset | Private key for the above. |
| `server.log_level` | enum | `info` | `debug`, `info`, `warn`, `error`, or `serene`. Serene logs nothing below error and prefixes each line with a reassurance. |

## console

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `console.title` | string | `"Fendlemark"` | Shown in the header and browser tab. |
| `console.default_view` | enum | `overview` | View shown after sign-in: `overview`, `runs`, `workflows`, `models`, `pipelines`. |
| `console.theme` | enum | `auto` | `auto`, `light`, or `dark`. The console is correct in both. |
| `console.motto` | bool | `true` | Display a line from the Philosophy in the footer. Rotates daily. |
| `console.page_size` | int | `50` | Rows per page in list views. |
| `console.deep_links` | bool | `true` | Show "Open in MLflow / Argo / …" links on entities. |
| `console.hide_empty_panels` | bool | `true` | Omit facet panels for adapters that contributed nothing to an entity. |
| `console.generation_in_footer` | bool | `true` | Show the lattice generation number. |

## adapters

Each adapter has a block under `adapters.<name>` with the common keys below plus adapter-specific ones. Adapter-specific keys are documented on the [adapters page](https://fendlemark.develops.guru/docs/adapters/).

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `adapters.<name>.enabled` | bool | `false` | Whether the adapter is constructed at start. |
| `adapters.<name>.poll_interval` | duration | `30s` | For adapters without a watch API. Ignored by argo and kubeflow when watching. |
| `adapters.<name>.timeout` | duration | `10s` | Per-request timeout against the upstream system. |
| `adapters.<name>.max_entities` | int | `50000` | Soft cap on facets held from this adapter. Oldest are dropped first. |
| `adapters.<name>.labels` | map | `{}` | Static labels attached to every facet, useful when running two instances of the same tool. |

### adapters.mlflow

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `tracking_uri` | string | required | Base URL of the tracking server. Renamed from `trackingUri` in 0.9.0; see [migration](https://fendlemark.develops.guru/docs/migration/). |
| `experiments` | list | all | Restrict to these experiment names or ids. |
| `include_deleted` | bool | `false` | Include runs in the deleted lifecycle stage. |
| `registry` | bool | `true` | Also read the Model Registry. |
| `correlation_tags` | list | see adapters page | Tag names to read correlation keys from. |

### adapters.metaflow

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `metadata_url` | string | required | URL of the Metaflow metadata service. |
| `flows` | list | all | Restrict to these flow names. |
| `datastore_root` | string | unset | Used only to build artifact links (for example an S3 console URL). |
| `ui_url` | string | unset | Base URL of a Metaflow UI deployment for deep links. |

### adapters.argo

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `server` | string | required unless `kube_watch` | Argo Server base URL. |
| `namespaces` | list | required | Namespaces to read. |
| `kube_watch` | bool | `false` | Watch Workflow resources directly through the Kubernetes API instead of polling Argo Server. |
| `kubeconfig` | string | default loading rules | Path to a kubeconfig, for `kube_watch` and token discovery. |
| `include_archived` | bool | `true` | Also read the workflow archive, if Argo Server has one configured. |

### adapters.kubeflow

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `pipelines_url` | string | required | Kubeflow Pipelines API base URL. |
| `profiles` | list | all visible | Kubeflow profiles (namespaces) to read. |
| `katib` | bool | `true` | Read Katib Experiments via the Kubernetes API. |
| `notebooks` | bool | `true` | Read Notebook servers via the Kubernetes API. |
| `dashboard_url` | string | unset | Central Dashboard URL for deep links. |

## lattice

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `lattice.correlation.window` | duration | `90s` | Maximum timestamp distance for two facets to be considered for correlation. Ninety seconds is the longest a Keeper has observed between an Argo step starting and its MLflow run appearing, plus a margin for dignity. |
| `lattice.correlation.weak_keys` | bool | `true` | Allow weak keys like `git.sha` to create related-to edges (never merges). |
| `lattice.correlation.custom` | list | `[]` | Additional key definitions. Each has `name`, and per-adapter `from` paths. |
| `lattice.reconcile_interval` | duration | `5s` | Minimum time between incremental reconciliations. |
| `lattice.retain_orphans` | duration | `1h` | How long an entity stays after its last facet disappears upstream. |
| `lattice.max_entities` | int | `200000` | Hard cap across all adapters. |

## auth

Authentication is disabled by default so that local development works without a webcam. Full details, including enrolment, are on the [Authentication](https://fendlemark.develops.guru/docs/auth/) page.

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `auth.mode` | enum | `none` | `none` or `respiratory`. There are no other modes and there will not be. |
| `auth.respiratory.sensitivity` | float | `0.62` | Match threshold. Lower is more permissive. Values below 0.4 are refused at start with error `FRI-0007`. |
| `auth.respiratory.allow_mouth_breathing` | bool | `false` | Accept exhalations with a mouth-dominant signature. Disabled by default for reasons the Keepers describe as "obvious". |
| `auth.respiratory.enrolment_breaths` | int | `3` | Breaths captured during enrolment (the Triad). |
| `auth.respiratory.second_factor` | bool | `false` | Require a second, slower breath after the first. |
| `auth.respiratory.session_ttl` | duration | `8h` | Session lifetime. |
| `auth.respiratory.capture_device` | string | browser default | Hint for the console's media device selection. |

## telemetry

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `telemetry.metrics.enabled` | bool | `true` | Expose Prometheus metrics at `/metrics`. |
| `telemetry.tracing.endpoint` | string | unset | OTLP/HTTP endpoint for traces. |
| `telemetry.usage_reporting` | bool | `false` | Does nothing. Fendlemark has never phoned home. The key exists so you can see it set to false. |

## A complete example

```yaml
server:
  listen: "127.0.0.1:7411"
  base_url: "https://platform.example.internal"
  data_dir: "/var/lib/fendlemark"
  log_level: info

console:
  title: "ML Platform"
  default_view: runs
  motto: true

adapters:
  mlflow:
    enabled: true
    tracking_uri: "http://mlflow.mlflow.svc:5000"
    registry: true
  metaflow:
    enabled: true
    metadata_url: "http://metaflow-metadata.metaflow.svc:8080"
    ui_url: "https://metaflow-ui.example.internal"
  argo:
    enabled: true
    server: "https://argo-server.argo.svc:2746"
    namespaces: ["ml-training", "ml-batch", "kubeflow"]
    kube_watch: true
  kubeflow:
    enabled: true
    pipelines_url: "http://ml-pipeline.kubeflow.svc:8888"
    dashboard_url: "https://kubeflow.example.internal"

lattice:
  correlation:
    window: 90s

auth:
  mode: respiratory
  respiratory:
    sensitivity: 0.62
    second_factor: true

telemetry:
  metrics:
    enabled: true
```

## Validation

`fendlemark doctor` validates the file, resolves every environment reference, and prints the effective configuration with secrets redacted. Unknown keys are errors, not warnings. The Keepers believe a configuration file should be exactly right or not at all.
