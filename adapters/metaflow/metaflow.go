// Package metaflow reads flows, runs, steps, and tasks from the Metaflow
// metadata service, the same service the Metaflow client and UI read.
//
// See https://fendlemark.app/docs/adapters/#metaflow for its facets,
// correlation keys, and limits.
package metaflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapters/internal/httpx"
)

func init() { adapter.Register("metaflow", New) }

// Metaflow is the adapter.
type Metaflow struct {
	mu        sync.RWMutex
	c         *httpx.Client
	flows     []string
	datastore string
	ui        string
	labels    map[string]string
}

func New() adapter.Adapter { return &Metaflow{} }

func (m *Metaflow) Name() string { return "metaflow" }

func (m *Metaflow) Describe() adapter.Descriptor {
	return adapter.Descriptor{
		DisplayName: "Metaflow",
		Kinds:       []adapter.Kind{adapter.KindRun, adapter.KindPipeline},
		Keys:        []string{adapter.KeyMetaflowRunID, adapter.KeyMetaflowFlow, adapter.KeyArgoWorkflow, adapter.KeyGitSHA, adapter.KeyGitRepo},
		Watchable:   false,
		Maturity:    adapter.Canonical,
		ConfigSchema: json.RawMessage(`{"type":"object","required":["metadata_url"],"properties":{
			"metadata_url":{"type":"string"},"flows":{"type":"array","items":{"type":"string"}},
			"datastore_root":{"type":"string"},"ui_url":{"type":"string"}}}`),
	}
}

func (m *Metaflow) Connect(ctx context.Context, cfg adapter.Config) error {
	u := cfg.String("metadata_url")
	if u == "" {
		return fmt.Errorf("metaflow: metadata_url is required")
	}
	c, err := httpx.New(u, cfg.Duration("timeout", 10*time.Second))
	if err != nil {
		return fmt.Errorf("metaflow: %w", err)
	}
	if tok := cfg.Secret("token"); tok != "" {
		c.Bearer = tok
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.c = c
	m.flows = cfg.Strings("flows")
	m.datastore = strings.TrimRight(cfg.String("datastore_root"), "/")
	m.ui = strings.TrimRight(cfg.String("ui_url"), "/")
	m.labels = map[string]string{}
	if l, ok := cfg["labels"].(map[string]any); ok {
		for k, v := range l {
			m.labels[k] = fmt.Sprint(v)
		}
	}
	return nil
}

type flow struct {
	FlowID  string `json:"flow_id"`
	User    string `json:"user_name"`
	TSEpoch int64  `json:"ts_epoch"`
}

// runNumber is a run's id as the metadata service reports it: a number for
// runs started locally (1824) and a string for runs started by an
// orchestrator such as Argo Workflows ("argo-trainflow-xyz99").
type runNumber string

func (n *runNumber) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*n = runNumber(s)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(b, &num); err != nil {
		return fmt.Errorf("run_number is neither a number nor a string: %s", b)
	}
	*n = runNumber(num.String())
	return nil
}

type runRec struct {
	FlowID     string    `json:"flow_id"`
	RunNumber  runNumber `json:"run_number"`
	RunID      string    `json:"run_id"` // older metadata services sent an orchestrator's run id here
	User       string    `json:"user_name"`
	TSEpoch    int64     `json:"ts_epoch"`
	Tags       []string  `json:"tags"`
	SystemTags []string  `json:"system_tags"`
	LastHB     float64   `json:"last_heartbeat_ts"`
}

func (m *Metaflow) Facets(ctx context.Context, q adapter.Query) ([]adapter.Facet, error) {
	m.mu.RLock()
	c, want, ds, ui, labels := m.c, m.flows, m.datastore, m.ui, m.labels
	m.mu.RUnlock()
	if c == nil {
		return nil, adapter.ErrNotConnected
	}
	var flows []flow
	if err := c.GetJSON(ctx, "/flows", nil, &flows); err != nil {
		return nil, fmt.Errorf("metaflow: /flows: %w", err)
	}
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
	}
	var out []adapter.Facet
	for _, f := range flows {
		if len(wanted) > 0 && !wanted[f.FlowID] {
			continue
		}
		if q.Wants(adapter.KindPipeline) {
			out = append(out, adapter.Facet{
				Source: "metaflow", SourceID: "flow:" + f.FlowID, Kind: adapter.KindPipeline, Title: f.FlowID,
				Status: adapter.Succeeded, StartedAt: epoch(f.TSEpoch),
				Keys:   map[string]string{adapter.KeyMetaflowFlow: f.FlowID},
				Fields: map[string]any{"user": f.User},
				Links:  uiLinks(ui, f.FlowID, ""),
			})
		}
		if !q.Wants(adapter.KindRun) {
			continue
		}
		var runs []runRec
		if err := c.GetJSON(ctx, "/flows/"+url.PathEscape(f.FlowID)+"/runs", nil, &runs); err != nil {
			return nil, fmt.Errorf("metaflow: runs of %s: %w", f.FlowID, err)
		}
		for _, r := range runs {
			started := epoch(r.TSEpoch)
			if !q.Since.IsZero() && started.Before(q.Since) {
				continue
			}
			runID := r.RunID
			if runID == "" {
				runID = string(r.RunNumber)
			}
			keys := map[string]string{adapter.KeyMetaflowRunID: f.FlowID + "/" + runID, adapter.KeyMetaflowFlow: f.FlowID}
			fields := map[string]any{"flow": f.FlowID, "run": runID, "user": r.User, "tags": r.Tags, "system_tags": r.SystemTags}
			for k, v := range labels {
				fields["label."+k] = v
			}
			for _, t := range r.SystemTags {
				switch {
				case strings.HasPrefix(t, "argo-workflows:"):
					keys[adapter.KeyArgoWorkflow] = strings.TrimPrefix(t, "argo-workflows:")
				case strings.HasPrefix(t, "argo-workflow-name:"):
					keys[adapter.KeyArgoWorkflow] = strings.TrimPrefix(t, "argo-workflow-name:")
				}
			}
			for _, t := range append(append([]string{}, r.Tags...), r.SystemTags...) {
				switch {
				case strings.HasPrefix(t, "git.sha:"), strings.HasPrefix(t, "git_sha:"):
					keys[adapter.KeyGitSHA] = t[strings.Index(t, ":")+1:]
				case strings.HasPrefix(t, "git.repo:"), strings.HasPrefix(t, "git_repo:"):
					keys[adapter.KeyGitRepo] = t[strings.Index(t, ":")+1:]
				}
			}
			if ds != "" {
				fields["artifact_root"] = fmt.Sprintf("%s/%s/data/%s", ds, f.FlowID, runID)
			}
			// A run with a heartbeat in the last two minutes is running; the
			// metadata service does not store a terminal status for runs.
			status := adapter.Succeeded
			if r.LastHB > 0 && time.Since(time.Unix(int64(r.LastHB), 0)) < 2*time.Minute {
				status = adapter.Running
			}
			out = append(out, adapter.Facet{
				Source: "metaflow", SourceID: f.FlowID + "/" + runID, Kind: adapter.KindRun, Title: f.FlowID + " / " + runID,
				Status: status, StartedAt: started, Keys: keys, Fields: fields, Links: uiLinks(ui, f.FlowID, runID),
			})
		}
	}
	return out, nil
}

func uiLinks(ui, flow, run string) []adapter.Link {
	if ui == "" {
		return nil
	}
	if run == "" {
		return []adapter.Link{{Label: "Open in Metaflow UI", URL: ui + "/" + flow}}
	}
	return []adapter.Link{{Label: "Open in Metaflow UI", URL: ui + "/" + flow + "/" + run}}
}

func epoch(ms int64) time.Time {
	t := httpx.Millis(ms)
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

func (m *Metaflow) Watch(ctx context.Context) (<-chan adapter.Event, error) {
	return nil, adapter.ErrWatchUnsupported
}

func (m *Metaflow) Health(ctx context.Context) adapter.Status {
	m.mu.RLock()
	c := m.c
	m.mu.RUnlock()
	if c == nil {
		return adapter.Status{State: adapter.Disabled, Detail: "not connected"}
	}
	var flows []flow
	start := time.Now()
	err := c.GetJSON(ctx, "/flows", nil, &flows)
	lat := time.Since(start)
	if err != nil {
		return adapter.Status{State: adapter.Unreachable, Detail: err.Error(), Latency: lat}
	}
	return adapter.Status{State: adapter.Healthy, Detail: fmt.Sprintf("%d flows", len(flows)), Latency: lat}
}
