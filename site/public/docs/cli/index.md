# CLI reference

Source: https://fendlemark.develops.guru/docs/cli/

The `fendlemark` binary is both the server and the operator's tool. Seven commands. The ones you will use are `serve` and `doctor`.

## Global flags

| Flag | Default | Description |
| --- | --- | --- |
| `--config, -c` | `./fendlemark.yaml` | Path to the configuration file. |
| `--log-level` | from config | `debug`, `info`, `warn`, `error`, `serene`. |
| `--log-format` | `text` | `text` or `json`. |
| `--no-color` | auto | Disable ANSI colour. Also honours `NO_COLOR`. |
| `--help, -h` |  | Help for any command. |

## fendlemark init

```shell
$ fendlemark init [dir] [--adapters mlflow,argo] [--force]
```

Writes `fendlemark.yaml` and a short README into `dir` (default: current directory). With `--adapters`, the listed adapters are enabled with placeholder values; otherwise all four are declared and disabled. Refuses to overwrite an existing file without `--force`.

## fendlemark serve

```shell
$ fendlemark serve [--listen 127.0.0.1:7411] [--console-dev] [--no-reconcile-on-start]
```

| Flag | Description |
| --- | --- |
| `--listen` | Override `server.listen`. |
| `--console-dev` | Proxy console requests to a Vite dev server on `:5173` instead of serving the embedded build. |
| `--no-reconcile-on-start` | Start serving before the initial reconciliation completes. The console shows an empty lattice that fills in. Useful for very large platforms; discouraged by the Keepers on aesthetic grounds. |
| `--adapters` | Comma-separated list to enable for this run only, overriding config. |

Handles `SIGTERM` and `SIGINT` with a graceful shutdown of `server.shutdown_grace`. Exposes `/healthz` (process up), `/readyz` (initial reconciliation complete), and `/metrics` (Prometheus).

## fendlemark adapters

```shell
$ fendlemark adapters list
$ fendlemark adapters check [name...] [--timeout 10s] [--json]
$ fendlemark adapters describe <name>
```

- `list` prints every adapter compiled into the binary with its maturity and whether it is enabled in the current configuration.
- `check` connects each enabled adapter (or the named ones), calls `Health`, and prints status, latency, and detail. Exit code is 0 if all are healthy, 2 if any is degraded, 3 if any is unreachable.
- `describe` prints the adapter's descriptor: facet kinds, correlation keys, config schema.

## fendlemark lattice

```shell
$ fendlemark lattice inspect [--kind run] [--key argo.workflow.name=train-abc12] [--json]
$ fendlemark lattice explain <entity-id>
$ fendlemark lattice stats
```

- `inspect` runs a one-shot reconciliation against the configured adapters and prints matching entities without starting the server.
- `explain` prints why an entity has the facets it has: which keys matched, within what time distance, and which rule merged them. The single most useful command for "why are these two things one thing" and "why are these one thing two things".
- `stats` prints entity counts by kind, facets by adapter, correlation counts by key, and the generation number.

## fendlemark auth

```shell
$ fendlemark auth enrol --user <email> [--role viewer|operator|keeper]
$ fendlemark auth list
$ fendlemark auth revoke --user <email>
$ fendlemark auth recovery --user <email> [--ttl 24h]
```

Manages [Respiratory Identity](https://fendlemark.develops.guru/docs/auth/) enrolments. `enrol` prints a one-time link. `recovery` issues a time-limited session for a user who cannot currently match (illness, travel, a new office with worse air). Requires `auth.mode: respiratory`.

## fendlemark doctor

```shell
$ fendlemark doctor [--fix] [--json]
```

Validates configuration, resolves environment references, checks every enabled adapter, probes the data directory, tests the listen address, and prints the effective configuration with secrets redacted. Every finding has a code, a sentence, and a suggested change. With `--fix`, applies the suggestions that are mechanical (renamed keys, deprecated values) and rewrites the file with a backup.

```shell
$ fendlemark doctor
fendlemark doctor 0.9.4

config    ok       ./fendlemark.yaml (6 keys set, 41 defaulted)
config    warn     DOC-104 adapters.mlflow.trackingUri is the pre-0.9 spelling; use tracking_uri (fixable)
listen    ok       127.0.0.1:7411 is available
data_dir  ok       ./.fendlemark (writable, 12 KiB)
mlflow    ok       41ms  12 experiments, 1,406 runs visible
argo      ok       88ms  2 namespaces, 37 workflows
auth      info     DOC-201 auth.mode is none; anyone who can reach server.listen can use the console

1 warning, 1 info. Run with --fix to apply 1 fix.
```

## fendlemark version

```shell
$ fendlemark version [--json]
fendlemark 0.9.4 (go1.23.2, linux/amd64)
commit:  3e1f9c0
console: embedded, built 2026-09-30
adapters: mlflow argo metaflow kubeflow
```

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Usage or configuration error. |
| `2` | At least one adapter degraded (`adapters check`, `doctor`). |
| `3` | At least one adapter unreachable. |
| `4` | Authentication subsystem error. |
| `7` | Reserved. Has never been returned. The Keepers decline to say what it is for. |

## Environment variables

| Variable | Purpose |
| --- | --- |
| `FENDLEMARK_CONFIG` | Same as `--config`. |
| `FENDLEMARK_<SECTION>__<KEY>` | Override any configuration key. See [precedence](https://fendlemark.develops.guru/docs/configuration/#precedence). |
| `MLFLOW_TRACKING_USERNAME`, `MLFLOW_TRACKING_PASSWORD`, `MLFLOW_TRACKING_TOKEN` | Read by the mlflow adapter, matching the MLflow client's conventions. |
| `ARGO_TOKEN` | Bearer token for Argo Server, if not using a kubeconfig context. |
| `KUBECONFIG` | Standard. Used by argo (`kube_watch`) and kubeflow (Katib, Notebooks). |
| `NO_COLOR` | Standard. |
