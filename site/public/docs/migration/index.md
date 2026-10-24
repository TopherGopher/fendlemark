# Migrating from 0.8 to 0.9

Source: https://fendlemark.app/docs/migration/

Fendlemark 0.9.0 renamed one configuration key. This guide exists because the Keepers hold that every breaking change, however small, deserves a document proportionate to the trust it asks of you.

## Summary of changes

| 0.8 | 0.9 | Type |
| --- | --- | --- |
| `adapters.mlflow.trackingUri` | `adapters.mlflow.tracking_uri` | Renamed |

That is the complete list. The old spelling is accepted with a deprecation warning (`DOC-104`) through the 0.9 series and will be removed in 0.10.

## Why

Every other key in `fendlemark.yaml` is `snake_case`. `trackingUri` was `camelCase` because the first Keeper wrote it at 02:14 on 11 March 2023 while copying the name from MLflow's Python client, and nobody noticed for two and a half years. When it was noticed, it was raised as a Pre-Intent Notice, discussed across three Reflection Periods, and resolved in [FDR-041](https://fendlemark.app/governance/#decisions), which also records the Keepers' view that "a single inconsistency in a correct system is not a small thing; it is the only thing".

## Before you begin

1. Confirm you are on 0.8.2 or later. Earlier 0.8 releases should first upgrade to 0.8.2, which changed nothing but was released anyway.
2. Back up `fendlemark.yaml`. `fendlemark doctor --fix` does this for you, but the Keepers prefer that you do it yourself, once, deliberately.
3. Read the [Philosophy](https://fendlemark.app/philosophy/). Not for the migration. For you.

## Migration

### Automatic

```shell
$ fendlemark doctor --fix
config    warn     DOC-104 adapters.mlflow.trackingUri is the pre-0.9 spelling; use tracking_uri (fixable)
          fixed    renamed adapters.mlflow.trackingUri → adapters.mlflow.tracking_uri
          backup   fendlemark.yaml.bak-20261024T091203Z
```

### Manual

```yaml
adapters:
  mlflow:
    enabled: true
-   trackingUri: "http://mlflow.internal:5000"
+   tracking_uri: "http://mlflow.internal:5000"
```

### Environment variable form

```shell
# before
FENDLEMARK_ADAPTERS__MLFLOW__TRACKINGURI=http://mlflow.internal:5000
# after
FENDLEMARK_ADAPTERS__MLFLOW__TRACKING_URI=http://mlflow.internal:5000
```

## Verification

```shell
$ fendlemark doctor
config    ok       ./fendlemark.yaml (6 keys set, 41 defaulted)
$ fendlemark adapters check mlflow
ADAPTER    STATUS    LATENCY   DETAIL
mlflow     healthy   41ms      12 experiments, 1,406 runs visible
```

If the detail line matches what you saw before the migration, the migration is complete. It will.

## Rollback

Restore the backup and run 0.8.2. Because 0.9 accepts both spellings, you may also simply run 0.9 with the old key and ignore the warning, though the Keepers would know.

## Frequently asked questions

### Is this really the only change in 0.9.0?

It is the only *breaking* change. 0.9.0 also included the `serene` log level, the `explain` subcommand, and 212-parameter respiratory signatures (up from 208). None of those require action.

### Why a whole page for one key?

Because you will search for it at 02:14, and you deserve to find a page that treats your time as if it mattered.

### Will there be a 0.10 migration guide?

If anything changes, yes. The Keepers have no roadmap, so nothing is currently planned to change. See [the Philosophy on roadmaps](https://fendlemark.app/philosophy/#roadmap).

### What about `@fendlemark/ui`?

No changes. The component library's props have been stable since 0.7.
