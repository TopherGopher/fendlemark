// Package mlflow reads experiments, runs, and the Model Registry from an
// MLflow tracking server over its REST API. It never writes.
package mlflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapters/internal/httpx"
)

func init() { adapter.Register("mlflow", New) }

// DefaultCorrelationTags are the run tags read for correlation keys.
var DefaultCorrelationTags = map[string]string{
	"argo.workflow.name":        adapter.KeyArgoWorkflow,
	"fendlemark.argo_workflow":  adapter.KeyArgoWorkflow,
	"metaflow.run_id":           adapter.KeyMetaflowRunID,
	"metaflow.flow_name":        adapter.KeyMetaflowFlow,
	"kfp.run_id":                adapter.KeyKFPRunID,
	"mlflow.source.git.commit":  adapter.KeyGitSHA,
	"mlflow.source.git.repoURL": adapter.KeyGitRepo,
}

// MLflow is the adapter.
type MLflow struct {
	mu          sync.RWMutex
	c           *httpx.Client
	tracking    string
	experiments []string
	deleted     bool
	registry    bool
	tags        map[string]string
	labels      map[string]string
}

// New constructs an unconnected adapter.
func New() adapter.Adapter { return &MLflow{} }

func (m *MLflow) Name() string { return "mlflow" }

func (m *MLflow) Describe() adapter.Descriptor {
	return adapter.Descriptor{
		DisplayName: "MLflow",
		Kinds:       []adapter.Kind{adapter.KindRun, adapter.KindModel},
		Keys: []string{adapter.KeyMLflowRunID, adapter.KeyMLflowExperiment, adapter.KeyArgoWorkflow,
			adapter.KeyMetaflowRunID, adapter.KeyMetaflowFlow, adapter.KeyKFPRunID, adapter.KeyGitSHA, adapter.KeyGitRepo, "mlflow.model_version"},
		Watchable: false,
		Maturity:  adapter.Canonical,
		ConfigSchema: json.RawMessage(`{"type":"object","required":["tracking_uri"],"properties":{
			"tracking_uri":{"type":"string"},"experiments":{"type":"array","items":{"type":"string"}},
			"include_deleted":{"type":"boolean","default":false},"registry":{"type":"boolean","default":true},
			"correlation_tags":{"type":"array","items":{"type":"string"}}}}`),
	}
}

func (m *MLflow) Connect(ctx context.Context, cfg adapter.Config) error {
	uri := cfg.String("tracking_uri")
	if uri == "" {
		uri = cfg.String("trackingUri") // pre-0.9 spelling, see DOC-104
	}
	if uri == "" {
		return fmt.Errorf("mlflow: tracking_uri is required")
	}
	c, err := httpx.New(uri, cfg.Duration("timeout", 10*time.Second))
	if err != nil {
		return fmt.Errorf("mlflow: %w", err)
	}
	c.Bearer = firstNonEmpty(cfg.Secret("token"), os.Getenv("MLFLOW_TRACKING_TOKEN"))
	c.User = firstNonEmpty(cfg.String("username"), os.Getenv("MLFLOW_TRACKING_USERNAME"))
	c.Password = firstNonEmpty(cfg.Secret("password"), os.Getenv("MLFLOW_TRACKING_PASSWORD"))
	tags := map[string]string{}
	if custom := cfg.Strings("correlation_tags"); len(custom) > 0 {
		for _, t := range custom {
			if k, ok := DefaultCorrelationTags[t]; ok {
				tags[t] = k
			} else {
				tags[t] = t
			}
		}
	} else {
		for k, v := range DefaultCorrelationTags {
			tags[k] = v
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.c, m.tracking = c, uri
	m.experiments = cfg.Strings("experiments")
	m.deleted = cfg.Bool("include_deleted", false)
	m.registry = cfg.Bool("registry", true)
	m.tags = tags
	m.labels = stringMap(cfg["labels"])
	return nil
}

type experiment struct {
	ExperimentID string `json:"experiment_id"`
	Name         string `json:"name"`
	Lifecycle    string `json:"lifecycle_stage"`
}

type run struct {
	Info struct {
		RunID        string `json:"run_id"`
		RunName      string `json:"run_name"`
		ExperimentID string `json:"experiment_id"`
		Status       string `json:"status"`
		StartTime    int64  `json:"start_time"`
		EndTime      int64  `json:"end_time"`
		ArtifactURI  string `json:"artifact_uri"`
		Lifecycle    string `json:"lifecycle_stage"`
		UserID       string `json:"user_id"`
	} `json:"info"`
	Data struct {
		Metrics []struct {
			Key   string  `json:"key"`
			Value float64 `json:"value"`
			Step  int64   `json:"step"`
		} `json:"metrics"`
		Params []kv `json:"params"`
		Tags   []kv `json:"tags"`
	} `json:"data"`
}

type kv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type modelVersion struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Stage       string   `json:"current_stage"`
	Status      string   `json:"status"`
	RunID       string   `json:"run_id"`
	Source      string   `json:"source"`
	Created     int64    `json:"creation_timestamp"`
	Updated     int64    `json:"last_updated_timestamp"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
}

func (m *MLflow) Facets(ctx context.Context, q adapter.Query) ([]adapter.Facet, error) {
	m.mu.RLock()
	c, exps, deleted, registry, tags, labels, tracking := m.c, m.experiments, m.deleted, m.registry, m.tags, m.labels, m.tracking
	m.mu.RUnlock()
	if c == nil {
		return nil, adapter.ErrNotConnected
	}
	var out []adapter.Facet
	if q.Wants(adapter.KindRun) {
		ids, names, err := m.experimentIDs(ctx, c, exps)
		if err != nil {
			return nil, err
		}
		viewType := "ACTIVE_ONLY"
		if deleted {
			viewType = "ALL"
		}
		filter := ""
		if !q.Since.IsZero() {
			filter = fmt.Sprintf("attributes.start_time >= %d", q.Since.UnixMilli())
		}
		token := ""
		// With no experiment to read there are no runs, and runs/search
		// rejects an empty experiment_ids list.
		for len(ids) > 0 {
			var resp struct {
				Runs      []run  `json:"runs"`
				NextToken string `json:"next_page_token"`
			}
			body := map[string]any{"experiment_ids": ids, "max_results": 1000, "run_view_type": viewType, "order_by": []string{"attributes.start_time DESC"}}
			if filter != "" {
				body["filter"] = filter
			}
			if token != "" {
				body["page_token"] = token
			}
			if err := c.PostJSON(ctx, "/api/2.0/mlflow/runs/search", body, &resp); err != nil {
				return nil, fmt.Errorf("mlflow: runs/search: %w", err)
			}
			for _, r := range resp.Runs {
				out = append(out, m.runFacet(r, names, tags, labels, tracking))
			}
			if resp.NextToken == "" || (q.Limit > 0 && len(out) >= q.Limit) {
				break
			}
			token = resp.NextToken
		}
	}
	if registry && q.Wants(adapter.KindModel) {
		token := ""
		for {
			var resp struct {
				Versions  []modelVersion `json:"model_versions"`
				NextToken string         `json:"next_page_token"`
			}
			qv := map[string][]string{"max_results": {"200"}}
			if token != "" {
				qv["page_token"] = []string{token}
			}
			if err := c.GetJSON(ctx, "/api/2.0/mlflow/model-versions/search", qv, &resp); err != nil {
				return nil, fmt.Errorf("mlflow: model-versions/search: %w", err)
			}
			for _, v := range resp.Versions {
				out = append(out, m.modelFacet(v, labels, tracking))
			}
			if resp.NextToken == "" {
				break
			}
			token = resp.NextToken
		}
	}
	return out, nil
}

func (m *MLflow) experimentIDs(ctx context.Context, c *httpx.Client, want []string) ([]string, map[string]string, error) {
	var ids []string
	names := map[string]string{}
	token := ""
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
	}
	for {
		var resp struct {
			Experiments []experiment `json:"experiments"`
			NextToken   string       `json:"next_page_token"`
		}
		body := map[string]any{"max_results": 1000, "view_type": "ACTIVE_ONLY"}
		if token != "" {
			body["page_token"] = token
		}
		if err := c.PostJSON(ctx, "/api/2.0/mlflow/experiments/search", body, &resp); err != nil {
			return nil, nil, fmt.Errorf("mlflow: experiments/search: %w", err)
		}
		for _, e := range resp.Experiments {
			if len(wanted) > 0 && !wanted[e.Name] && !wanted[e.ExperimentID] {
				continue
			}
			ids = append(ids, e.ExperimentID)
			names[e.ExperimentID] = e.Name
		}
		if resp.NextToken == "" {
			break
		}
		token = resp.NextToken
	}
	return ids, names, nil
}

func (m *MLflow) runFacet(r run, names map[string]string, tags, labels map[string]string, tracking string) adapter.Facet {
	keys := map[string]string{adapter.KeyMLflowRunID: r.Info.RunID, adapter.KeyMLflowExperiment: r.Info.ExperimentID}
	fields := map[string]any{"experiment": names[r.Info.ExperimentID], "user": r.Info.UserID, "artifact_uri": r.Info.ArtifactURI, "lifecycle_stage": r.Info.Lifecycle}
	params := map[string]string{}
	for _, p := range r.Data.Params {
		params[p.Key] = p.Value
	}
	metrics := map[string]float64{}
	for _, mt := range r.Data.Metrics {
		metrics[mt.Key] = mt.Value
	}
	tagMap := map[string]string{}
	for _, t := range r.Data.Tags {
		tagMap[t.Key] = t.Value
		if k, ok := tags[t.Key]; ok && t.Value != "" {
			keys[k] = t.Value
		}
	}
	for k, v := range labels {
		fields["label."+k] = v
	}
	fields["params"], fields["metrics"], fields["tags"] = params, metrics, tagMap
	title := r.Info.RunName
	if title == "" {
		title = tagMap["mlflow.runName"]
	}
	if title == "" {
		title = r.Info.RunID[:min(8, len(r.Info.RunID))]
	}
	started := httpx.Millis(r.Info.StartTime)
	if started.IsZero() {
		started = time.Now().UTC()
	}
	return adapter.Facet{
		Source: "mlflow", SourceID: r.Info.RunID, Kind: adapter.KindRun, Title: title,
		Status: phase(r.Info.Status), StartedAt: started, EndedAt: httpx.Millis(r.Info.EndTime),
		Keys: keys, Fields: fields,
		Links: []adapter.Link{{Label: "Open in MLflow", URL: fmt.Sprintf("%s/#/experiments/%s/runs/%s", strings.TrimRight(tracking, "/"), r.Info.ExperimentID, r.Info.RunID)}},
	}
}

func (m *MLflow) modelFacet(v modelVersion, labels map[string]string, tracking string) adapter.Facet {
	id := "model:" + v.Name + ":v" + v.Version
	fields := map[string]any{"name": v.Name, "version": v.Version, "stage": v.Stage, "status": v.Status, "source": v.Source, "description": v.Description, "aliases": v.Aliases}
	if v.RunID != "" {
		fields["source_run_id"] = v.RunID
	}
	for k, l := range labels {
		fields["label."+k] = l
	}
	st := adapter.Succeeded
	if v.Status != "" && v.Status != "READY" {
		st = adapter.Pending
	}
	started := httpx.Millis(v.Created)
	if started.IsZero() {
		started = time.Now().UTC()
	}
	return adapter.Facet{
		Source: "mlflow", SourceID: id, Kind: adapter.KindModel, Title: v.Name + " v" + v.Version,
		Status: st, StartedAt: started, EndedAt: httpx.Millis(v.Updated),
		Keys:   map[string]string{"mlflow.model_version": v.Name + ":" + v.Version},
		Fields: fields,
		Links:  []adapter.Link{{Label: "Open in MLflow", URL: fmt.Sprintf("%s/#/models/%s/versions/%s", strings.TrimRight(tracking, "/"), v.Name, v.Version)}},
	}
}

func phase(s string) adapter.Phase {
	switch strings.ToUpper(s) {
	case "RUNNING":
		return adapter.Running
	case "FINISHED":
		return adapter.Succeeded
	case "FAILED", "KILLED":
		return adapter.Failed
	case "SCHEDULED":
		return adapter.Pending
	}
	return adapter.Unknown
}

func (m *MLflow) Watch(ctx context.Context) (<-chan adapter.Event, error) {
	return nil, adapter.ErrWatchUnsupported
}

func (m *MLflow) Health(ctx context.Context) adapter.Status {
	m.mu.RLock()
	c := m.c
	m.mu.RUnlock()
	if c == nil {
		return adapter.Status{State: adapter.Disabled, Detail: "not connected"}
	}
	var resp struct {
		Experiments []experiment `json:"experiments"`
	}
	start := time.Now()
	err := c.PostJSON(ctx, "/api/2.0/mlflow/experiments/search", map[string]any{"max_results": 1}, &resp)
	lat := time.Since(start)
	if err != nil {
		return adapter.Status{State: adapter.Unreachable, Detail: err.Error(), Latency: lat}
	}
	return adapter.Status{State: adapter.Healthy, Detail: "tracking server reachable", Latency: lat}
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func stringMap(v any) map[string]string {
	out := map[string]string{}
	switch t := v.(type) {
	case map[string]string:
		return t
	case map[string]any:
		for k, e := range t {
			out[k] = fmt.Sprint(e)
		}
	}
	return out
}
