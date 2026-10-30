package kubeflow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapters/internal/httpx"
	"fendlemark.app/fendlemark/adapters/internal/kube"
)

type meta struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
}

type condition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// current is the type of the last condition that holds, as Katib reports
// an object's progress by appending conditions.
func current(cs []condition) string {
	for i := len(cs) - 1; i >= 0; i-- {
		if cs[i].Status == "True" {
			return cs[i].Type
		}
	}
	return ""
}

type experiment struct {
	Metadata meta `json:"metadata"`
	Spec     struct {
		Objective struct {
			Type       string   `json:"type"`
			Goal       *float64 `json:"goal"`
			MetricName string   `json:"objectiveMetricName"`
		} `json:"objective"`
		Algorithm struct {
			Name string `json:"algorithmName"`
		} `json:"algorithm"`
		MaxTrialCount int `json:"maxTrialCount"`
	} `json:"spec"`
	Status struct {
		StartTime      string      `json:"startTime"`
		CompletionTime string      `json:"completionTime"`
		Conditions     []condition `json:"conditions"`
		Trials         int         `json:"trials"`
		Succeeded      int         `json:"trialsSucceeded"`
		Failed         int         `json:"trialsFailed"`
		Running        int         `json:"trialsRunning"`
		Optimal        struct {
			BestTrialName string `json:"bestTrialName"`
			Assignments   []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"parameterAssignments"`
			Observation struct {
				Metrics []struct {
					Name   string `json:"name"`
					Latest string `json:"latest"`
				} `json:"metrics"`
			} `json:"observation"`
		} `json:"currentOptimalTrial"`
	} `json:"status"`
}

type trial struct {
	Metadata meta `json:"metadata"`
	Status   struct {
		Conditions []condition `json:"conditions"`
	} `json:"status"`
}

func katibPhase(s string) adapter.Phase {
	switch s {
	case "Created":
		return adapter.Pending
	case "Running", "Restarting":
		return adapter.Running
	case "Succeeded", "EarlyStopped":
		return adapter.Succeeded
	case "Failed", "Killed", "MetricsUnavailable":
		return adapter.Failed
	}
	return adapter.Unknown
}

// katibFacets reads Katib Experiments and their Trials in one namespace, or
// in all of them when ns is empty. A cluster without Katib contributes none.
func katibFacets(ctx context.Context, kc *kube.Client, ns string, labels map[string]string) ([]adapter.Facet, error) {
	var exps struct {
		Items []experiment `json:"items"`
	}
	ok, err := kc.List(ctx, "kubeflow.org", "v1beta1", ns, "experiments", nil, &exps)
	if err != nil || !ok {
		return nil, err
	}
	var trials struct {
		Items []trial `json:"items"`
	}
	if _, err := kc.List(ctx, "kubeflow.org", "v1beta1", ns, "trials", nil, &trials); err != nil {
		return nil, err
	}
	byExp := map[string][]map[string]string{}
	for _, t := range trials.Items {
		k := t.Metadata.Namespace + "/" + t.Metadata.Labels["katib.kubeflow.org/experiment"]
		byExp[k] = append(byExp[k], map[string]string{"name": t.Metadata.Name, "phase": string(katibPhase(current(t.Status.Conditions)))})
	}
	out := make([]adapter.Facet, 0, len(exps.Items))
	for _, e := range exps.Items {
		id := e.Metadata.Namespace + "/" + e.Metadata.Name
		objective := e.Spec.Objective.Type + " " + e.Spec.Objective.MetricName
		if e.Spec.Objective.Goal != nil {
			objective += fmt.Sprintf(" (goal %g)", *e.Spec.Objective.Goal)
		}
		best := map[string]any{}
		if o := e.Status.Optimal; o.BestTrialName != "" {
			params, metrics := map[string]string{}, map[string]string{}
			for _, a := range o.Assignments {
				params[a.Name] = a.Value
			}
			for _, m := range o.Observation.Metrics {
				metrics[m.Name] = m.Latest
			}
			best = map[string]any{"name": o.BestTrialName, "parameters": params, "metrics": metrics}
		}
		ts := byExp[id]
		sort.Slice(ts, func(i, j int) bool { return ts[i]["name"] < ts[j]["name"] })
		fields := map[string]any{
			"namespace": e.Metadata.Namespace, "type": "KatibExperiment",
			"objective": strings.TrimSpace(objective), "algorithm": e.Spec.Algorithm.Name,
			"max_trials": e.Spec.MaxTrialCount, "trials": e.Status.Trials, "trials_succeeded": e.Status.Succeeded,
			"trials_failed": e.Status.Failed, "trials_running": e.Status.Running,
			"best_trial": best, "trial_phases": ts,
		}
		for key, v := range labels {
			fields["label."+key] = v
		}
		started := httpx.ParseTime(e.Status.StartTime)
		if started.IsZero() {
			started = orNow(httpx.ParseTime(e.Metadata.CreationTimestamp))
		}
		out = append(out, adapter.Facet{
			Source: "kubeflow", SourceID: "katib:" + id, Kind: adapter.KindWorkflow, Title: e.Metadata.Name,
			Status: katibPhase(current(e.Status.Conditions)), StartedAt: started, EndedAt: httpx.ParseTime(e.Status.CompletionTime),
			Fields: fields,
		})
	}
	return out, nil
}

type notebook struct {
	Metadata meta `json:"metadata"`
	Spec     struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Image     string         `json:"image"`
					Resources map[string]any `json:"resources"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas int `json:"readyReplicas"`
	} `json:"status"`
}

// notebookFacets reads Notebook servers in one namespace, or in all of them
// when ns is empty. A cluster without the Notebook controller contributes none.
func notebookFacets(ctx context.Context, kc *kube.Client, ns string, labels map[string]string, dash string) ([]adapter.Facet, error) {
	var nbs struct {
		Items []notebook `json:"items"`
	}
	ok, err := kc.List(ctx, "kubeflow.org", "v1", ns, "notebooks", nil, &nbs)
	if err != nil || !ok {
		return nil, err
	}
	out := make([]adapter.Facet, 0, len(nbs.Items))
	for _, n := range nbs.Items {
		state, ph := "starting", adapter.Pending
		switch {
		case n.Metadata.Annotations["kubeflow-resource-stopped"] != "":
			state = "stopped"
		case n.Status.ReadyReplicas > 0:
			state, ph = "running", adapter.Running
		}
		var image string
		var resources map[string]any
		if cs := n.Spec.Template.Spec.Containers; len(cs) > 0 {
			image, resources = cs[0].Image, cs[0].Resources
		}
		fields := map[string]any{"namespace": n.Metadata.Namespace, "type": "Notebook", "image": image, "resources": resources, "status": state}
		for key, v := range labels {
			fields["label."+key] = v
		}
		out = append(out, adapter.Facet{
			Source: "kubeflow", SourceID: "notebook:" + n.Metadata.Namespace + "/" + n.Metadata.Name, Kind: adapter.KindNotebook,
			Title: n.Metadata.Name, Status: ph, StartedAt: orNow(httpx.ParseTime(n.Metadata.CreationTimestamp)),
			Fields: fields, Links: dashLinks(dash, "/notebook/"+n.Metadata.Namespace+"/"+n.Metadata.Name+"/"),
		})
	}
	return out, nil
}
