package main

// The adapters compiled into this binary. Each package registers itself with
// adapter.Register when it is imported. To add your own, import it here for
// its side effect and rebuild:
//
//	_ "yourmodule/example"
import (
	_ "fendlemark.app/fendlemark/adapters/argo"
	_ "fendlemark.app/fendlemark/adapters/kubeflow"
	_ "fendlemark.app/fendlemark/adapters/metaflow"
	_ "fendlemark.app/fendlemark/adapters/mlflow"
)
