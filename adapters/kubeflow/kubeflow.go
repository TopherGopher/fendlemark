// Package kubeflow reads pipelines, runs, and recurring runs from the
// Kubeflow Pipelines API (v2beta1), with Katib and Notebooks disabled in
// this build unless a Kubernetes client is configured.
package kubeflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"fendlemark.develops.guru/fendlemark/adapter"
	"fendlemark.develops.guru/fendlemark/adapters/internal/httpx"
)

func init() { adapter.Register("kubeflow", New) }

// Kubeflow is the adapter.
type Kubeflow struct {
	mu        sync.RWMutex
	c         *httpx.Client
	profiles  []string
	dashboard string
	labels    map[string]string
	pollEvery time.Duration
}

func New() adapter.Adapter { return &Kubeflow{} }

func (k *Kubeflow) Name() string { return "kubeflow" }

func (k *Kubeflow) Describe() adapter.Descriptor {
	return adapter.Descriptor{
		DisplayName: "Kubeflow",
		Kinds:       []adapter.Kind{adapter.KindPipeline, adapter.KindWorkflow},
		Keys:        []string{adapter.KeyKFPRunID, adapter.KeyKFPPipelineID, adapter.KeyArgoWorkflow, adapter.KeyMetaflowRunID, adapter.KeyMLflowRunID, "kfp.recurring_run_id"},
		Watchable:   true,
		Maturity:    adapter.Canonical,
		ConfigSchema: json.RawMessage(`{"type":"object","required":["pipelines_url"],"properties":{
			"pipelines_url":{"type":"string"},"profiles":{"type":"array","items":{"type":"string"}},
			"katib":{"type":"boolean","default":true},"notebooks":{"type":"boolean","default":true},
			"dashboard_url":{"type":"string"}}}`),
	}
}

func (k *Kubeflow) Connect(ctx context.Context, cfg adapter.Config) error {
	u := cfg.String("pipelines_url")
	if u == "" {
		return fmt.Errorf("kubeflow: pipelines_url is required")
	}
	c, err := httpx.New(u, cfg.Duration("timeout", 10*time.Second))
	if err != nil {
		return fmt.Errorf("kubeflow: %w", err)
	}
	if tok := firstNonEmpty(cfg.Secret("token"), os.Getenv("KUBEFLOW_TOKEN")); tok != "" {
		c.Bearer = tok
	}
	if ns := cfg.String("user_namespace"); ns != "" {
		c.Header.Set("kubeflow-userid", cfg.String("user_id"))
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.c = c
	k.profiles = cfg.Strings("profiles")
	k.dashboard = strings.TrimRight(cfg.String("dashboard_url"), "/")
	k.pollEvery = cfg.Duration("poll_interval", 30*time.Second)
	k.labels = map[string]string{}
	if l, ok := cfg["labels"].(map[string]any); ok {
		for key, v := range l {
			k.labels[key] = fmt.Sprint(v)
		}
	}
	return nil
}

type run struct {
	RunID        string `json:"run_id"`
	DisplayName  string `json:"display_name"`
	ExperimentID string `json:"experiment_id"`
	Namespace    string `json:"namespace"`
	State        string `json:"state"`
	CreatedAt    string `json:"created_at"`
	ScheduledAt  string `json:"scheduled_at"`
	FinishedAt   string `json:"finished_at"`
	PipelineVer  struct {
		PipelineID        string `json:"pipeline_id"`
		PipelineVersionID string `json:"pipeline_version_id"`
	} `json:"pipeline_version_reference"`
	RuntimeConfig struct {
		Parameters map[string]any `json:"parameters"`
	} `json:"runtime_config"`
	RunDetails struct {
		PipelineContextID string `json:"pipeline_context_id"`
		TaskDetails       []struct {
			DisplayName string `json:"display_name"`
			State       string `json:"state"`
			StartTime   string `json:"start_time"`
			EndTime     string `json:"end_time"`
		} `json:"task_details"`
	} `json:"run_details"`
}

type pipeline struct {
	PipelineID  string `json:"pipeline_id"`
	DisplayName string `json:"display_name"`
	Namespace   string `json:"namespace"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description"`
}

type recurring struct {
	RecurringRunID string `json:"recurring_run_id"`
	DisplayName    string `json:"display_name"`
	Namespace      string `json:"namespace"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	Trigger        struct {
		CronSchedule struct {
			Cron string `json:"cron"`
		} `json:"cron_schedule"`
		PeriodicSchedule struct {
			IntervalSecond string `json:"interval_second"`
		} `json:"periodic_schedule"`
	} `json:"trigger"`
}

func (k *Kubeflow) Facets(ctx context.Context, q adapter.Query) ([]adapter.Facet, error) {
	k.mu.RLock()
	c, profiles, dash, labels := k.c, k.profiles, k.dashboard, k.labels
	k.mu.RUnlock()
	if c == nil {
		return nil, adapter.ErrNotConnected
	}
	scopes := profiles
	if len(scopes) == 0 {
		scopes = []string{""}
	}
	var out []adapter.Facet
	for _, ns := range scopes {
		if q.Wants(adapter.KindWorkflow) {
			token := ""
			for {
				var resp struct {
					Runs      []run  `json:"runs"`
					NextToken string `json:"next_page_token"`
				}
				qv := map[string][]string{"page_size": {"200"}, "sort_by": {"created_at desc"}}
				if ns != "" {
					qv["namespace"] = []string{ns}
				}
				if token != "" {
					qv["page_token"] = []string{token}
				}
				if err := c.GetJSON(ctx, "/apis/v2beta1/runs", qv, &resp); err != nil {
					return nil, fmt.Errorf("kubeflow: runs: %w", err)
				}
				for _, r := range resp.Runs {
					f := k.runFacet(r, labels, dash)
					if !q.Since.IsZero() && f.StartedAt.Before(q.Since) {
						continue
					}
					out = append(out, f)
				}
				if resp.NextToken == "" || (q.Limit > 0 && len(out) >= q.Limit) {
					break
				}
				token = resp.NextToken
			}
		}
		if q.Wants(adapter.KindPipeline) {
			var pl struct {
				Pipelines []pipeline `json:"pipelines"`
			}
			qv := map[string][]string{"page_size": {"200"}}
			if ns != "" {
				qv["namespace"] = []string{ns}
			}
			if err := c.GetJSON(ctx, "/apis/v2beta1/pipelines", qv, &pl); err != nil {
				return nil, fmt.Errorf("kubeflow: pipelines: %w", err)
			}
			for _, p := range pl.Pipelines {
				out = append(out, adapter.Facet{
					Source: "kubeflow", SourceID: "pipeline:" + p.PipelineID, Kind: adapter.KindPipeline, Title: p.DisplayName,
					Status: adapter.Succeeded, StartedAt: orNow(httpx.ParseTime(p.CreatedAt)),
					Keys:   map[string]string{adapter.KeyKFPPipelineID: p.PipelineID},
					Fields: map[string]any{"namespace": p.Namespace, "description": p.Description, "type": "Pipeline"},
					Links:  dashLinks(dash, "/pipeline/#/pipelines/details/"+p.PipelineID),
				})
			}
			var rr struct {
				RecurringRuns []recurring `json:"recurringRuns"`
			}
			if err := c.GetJSON(ctx, "/apis/v2beta1/recurringruns", qv, &rr); err == nil {
				for _, r := range rr.RecurringRuns {
					st := adapter.Succeeded
					if r.Status != "ENABLED" {
						st = adapter.Pending
					}
					sched := r.Trigger.CronSchedule.Cron
					if sched == "" && r.Trigger.PeriodicSchedule.IntervalSecond != "" {
						sched = "every " + r.Trigger.PeriodicSchedule.IntervalSecond + "s"
					}
					out = append(out, adapter.Facet{
						Source: "kubeflow", SourceID: "recurring:" + r.RecurringRunID, Kind: adapter.KindPipeline, Title: r.DisplayName,
						Status: st, StartedAt: orNow(httpx.ParseTime(r.CreatedAt)),
						Keys:   map[string]string{"kfp.recurring_run_id": r.RecurringRunID},
						Fields: map[string]any{"namespace": r.Namespace, "type": "RecurringRun", "status": r.Status, "schedule": sched},
						Links:  dashLinks(dash, "/pipeline/#/recurringrun/details/"+r.RecurringRunID),
					})
				}
			}
		}
	}
	return out, nil
}

func (k *Kubeflow) runFacet(r run, labels map[string]string, dash string) adapter.Facet {
	keys := map[string]string{adapter.KeyKFPRunID: r.RunID}
	if r.PipelineVer.PipelineID != "" {
		keys[adapter.KeyKFPPipelineID] = r.PipelineVer.PipelineID
	}
	params := map[string]any{}
	for name, v := range r.RuntimeConfig.Parameters {
		params[name] = v
		s, _ := v.(string)
		switch name {
		case "metaflow.run_id", "metaflow_run_id":
			if s != "" {
				keys[adapter.KeyMetaflowRunID] = s
			}
		case "mlflow.run_id", "mlflow_run_id":
			if s != "" {
				keys[adapter.KeyMLflowRunID] = s
			}
		}
	}
	// KFP names the underlying Argo Workflow after the run id, so the name is
	// recoverable without a second call. See the adapters documentation.
	if r.RunID != "" {
		keys[adapter.KeyArgoWorkflow] = r.RunID
	}
	tasks := make([]map[string]string, 0, len(r.RunDetails.TaskDetails))
	for _, t := range r.RunDetails.TaskDetails {
		tasks = append(tasks, map[string]string{"name": t.DisplayName, "phase": string(phase(t.State)), "started_at": t.StartTime, "ended_at": t.EndTime})
	}
	fields := map[string]any{"namespace": r.Namespace, "experiment_id": r.ExperimentID, "pipeline_version_id": r.PipelineVer.PipelineVersionID, "state": r.State, "parameters": params, "tasks": tasks}
	for key, v := range labels {
		fields["label."+key] = v
	}
	started := httpx.ParseTime(r.ScheduledAt)
	if started.IsZero() {
		started = orNow(httpx.ParseTime(r.CreatedAt))
	}
	return adapter.Facet{
		Source: "kubeflow", SourceID: "run:" + r.RunID, Kind: adapter.KindWorkflow, Title: r.DisplayName,
		Status: phase(r.State), StartedAt: started, EndedAt: httpx.ParseTime(r.FinishedAt),
		Keys: keys, Fields: fields, Links: dashLinks(dash, "/pipeline/#/runs/details/"+r.RunID),
	}
}

func phase(s string) adapter.Phase {
	switch strings.ToUpper(s) {
	case "PENDING", "SKIPPED":
		return adapter.Pending
	case "RUNNING", "CANCELING":
		return adapter.Running
	case "SUCCEEDED", "CACHED":
		return adapter.Succeeded
	case "FAILED", "CANCELED", "PAUSED":
		return adapter.Failed
	}
	return adapter.Unknown
}

func dashLinks(dash, path string) []adapter.Link {
	if dash == "" {
		return nil
	}
	return []adapter.Link{{Label: "Open in Kubeflow", URL: dash + path}}
}

// Watch polls runs and emits an event when a run's state or finish time changes.
func (k *Kubeflow) Watch(ctx context.Context) (<-chan adapter.Event, error) {
	k.mu.RLock()
	c, every := k.c, k.pollEvery
	k.mu.RUnlock()
	if c == nil {
		return nil, adapter.ErrNotConnected
	}
	if every <= 0 {
		every = 30 * time.Second
	}
	ch := make(chan adapter.Event, 256)
	go func() {
		defer close(ch)
		seen := map[string]string{}
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			facets, err := k.Facets(ctx, adapter.Query{Kinds: []adapter.Kind{adapter.KindWorkflow}})
			if err == nil {
				for _, f := range facets {
					sig := string(f.Status) + "|" + f.EndedAt.Format(time.RFC3339)
					if seen[f.Ref()] != sig {
						seen[f.Ref()] = sig
						select {
						case ch <- adapter.Event{Op: adapter.Upsert, Facet: f}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return ch, nil
}

func (k *Kubeflow) Health(ctx context.Context) adapter.Status {
	k.mu.RLock()
	c := k.c
	k.mu.RUnlock()
	if c == nil {
		return adapter.Status{State: adapter.Disabled, Detail: "not connected"}
	}
	start := time.Now()
	var hc map[string]any
	if err := c.GetJSON(ctx, "/apis/v2beta1/healthz", nil, &hc); err != nil {
		return adapter.Status{State: adapter.Unreachable, Detail: err.Error(), Latency: time.Since(start)}
	}
	return adapter.Status{State: adapter.Healthy, Detail: "pipelines API reachable", Latency: time.Since(start)}
}

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
