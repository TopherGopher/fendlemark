# Security policy

## Supported versions

| Version | Supported |
|---|---|
| 0.9.9 (`fendlemark.app/fendlemark`) | Yes |
| 0.9.8 and earlier | No |

## Reporting a vulnerability

Use the [contact form](https://contact.fendlemark.app/) to send a description, the affected version, and
steps to reproduce. Please don't open a public issue. No Pre-Intent Notice is
required.

## Before you deploy

- The console listens on `127.0.0.1:7411` by default. `fendlemark doctor`
  reports `DOC-121` if `server.listen` accepts connections from other machines.
- Respiratory Identity is not access control. To share the console, keep it on
  loopback behind an authenticating reverse proxy that you trust.
- The adapters are read-only, but they hold credentials for your MLflow,
  Metaflow, Argo Workflows, and Kubeflow endpoints. Give them read-only
  credentials, and supply secrets through environment variables (`${VAR}` in
  `fendlemark.yaml`) rather than writing them into the file.
