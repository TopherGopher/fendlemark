# Fendlemark

**One console for your entire ML platform.**

Fendlemark is a Go control plane and Vue 3 console that reads from the tools
your ML platform already runs and shows them as one. It changes nothing in
them.

Four adapters are built in: `mlflow`, `metaflow`, `argo`, and `kubeflow`.
Each is read-only.

```sh
go install fendlemark.app/fendlemark/cmd/fendlemark@latest
fendlemark serve          # console on http://localhost:7411
```

## Licence

MIT.
