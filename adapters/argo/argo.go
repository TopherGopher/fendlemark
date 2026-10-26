// Package argo reads Workflows, WorkflowTemplates, and CronWorkflows from
// Argo Server's REST API, with optional direct watching of Workflow
// resources through the Kubernetes API.
//
// See https://fendlemark.app/docs/adapters/#argo for its facets,
// correlation keys, and limits.
package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"fendlemark.app/fendlemark/adapter"
	"fendlemark.app/fendlemark/adapters/internal/httpx"
)

func init() { adapter.Register("argo", New) }

// Argo is the adapter.
type Argo struct {
	mu         sync.RWMutex
	c          *httpx.Client
	server     string
	namespaces []string
	archived   bool
	kubeWatch  bool
	labels     map[string]string
	pollEvery  time.Duration
}

func New() adapter.Adapter { return &Argo{} }

func (a *Argo) Name() string { return "argo" }

func (a *Argo) Describe() adapter.Descriptor {
	return adapter.Descriptor{
		DisplayName: "Argo Workflows",
		Kinds:       []adapter.Kind{adapter.KindWorkflow, adapter.KindPipeline},
		Keys:        []string{adapter.KeyArgoWorkflow, adapter.KeyArgoUID, adapter.KeyKFPRunID, adapter.KeyMetaflowRunID, adapter.KeyGitSHA, adapter.KeyGitRepo, "argo.template", "argo.cron"},
		Watchable:   true,
		Maturity:    adapter.Canonical,
		ConfigSchema: json.RawMessage(`{"type":"object","required":["namespaces"],"properties":{
			"server":{"type":"string"},"namespaces":{"type":"array","items":{"type":"string"}},
			"kube_watch":{"type":"boolean","default":false},"kubeconfig":{"type":"string"},
			"include_archived":{"type":"boolean","default":true}}}`),
	}
}

func (a *Argo) Connect(ctx context.Context, cfg adapter.Config) error {
	srv := cfg.String("server")
	ns := cfg.Strings("namespaces")
	if len(ns) == 0 {
		return fmt.Errorf("argo: namespaces must list at least one namespace")
	}
	if srv == "" {
		return fmt.Errorf("argo: server is required (direct kube_watch without Argo Server is not available in this build)")
	}
	c, err := httpx.New(srv, cfg.Duration("timeout", 10*time.Second))
	if err != nil {
		return fmt.Errorf("argo: %w", err)
	}
	if tok := firstNonEmpty(cfg.Secret("token"), os.Getenv("ARGO_TOKEN")); tok != "" {
		c.Bearer = strings.TrimPrefix(tok, "Bearer ")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.c, a.server, a.namespaces = c, strings.TrimRight(srv, "/"), ns
	a.archived = cfg.Bool("include_archived", true)
	a.kubeWatch = cfg.Bool("kube_watch", false)
	a.pollEvery = cfg.Duration("poll_interval", 30*time.Second)
	a.labels = map[string]string{}
	if l, ok := cfg["labels"].(map[string]any); ok {
		for k, v := range l {
			a.labels[k] = fmt.Sprint(v)
		}
	}
	return nil
}

type workflow struct {
	Metadata struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		UID         string            `json:"uid"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Created     string            `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Entrypoint string `json:"entrypoint"`
		Arguments  struct {
			Parameters []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"parameters"`
		} `json:"arguments"`
		WorkflowTemplateRef struct {
			Name string `json:"name"`
		} `json:"workflowTemplateRef"`
	} `json:"spec"`
	Status struct {
		Phase      string `json:"phase"`
		Message    string `json:"message"`
		StartedAt  string `json:"startedAt"`
		FinishedAt string `json:"finishedAt"`
		Progress   string `json:"progress"`
		Nodes      map[string]struct {
			Name         string `json:"name"`
			DisplayName  string `json:"displayName"`
			Type         string `json:"type"`
			Phase        string `json:"phase"`
			TemplateName string `json:"templateName"`
			StartedAt    string `json:"startedAt"`
			FinishedAt   string `json:"finishedAt"`
		} `json:"nodes"`
	} `json:"status"`
}

type list struct {
	Items    []workflow `json:"items"`
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
}

func (a *Argo) Facets(ctx context.Context, q adapter.Query) ([]adapter.Facet, error) {
	a.mu.RLock()
	c, nss, archived, labels, server := a.c, a.namespaces, a.archived, a.labels, a.server
	a.mu.RUnlock()
	if c == nil {
		return nil, adapter.ErrNotConnected
	}
	var out []adapter.Facet
	for _, ns := range nss {
		if q.Wants(adapter.KindWorkflow) {
			// A workflow can be both live and archived. The live copy has the
			// spec and the node status, so it wins; the archive only adds
			// workflows that are no longer in the cluster.
			listed := map[string]bool{}
			cont := ""
			for {
				var l list
				qv := map[string][]string{"listOptions.limit": {"500"}, "fields": {"items.metadata,items.spec.entrypoint,items.spec.arguments,items.spec.workflowTemplateRef,items.status.phase,items.status.message,items.status.startedAt,items.status.finishedAt,items.status.progress,items.status.nodes,metadata.continue"}}
				if cont != "" {
					qv["listOptions.continue"] = []string{cont}
				}
				if err := c.GetJSON(ctx, "/api/v1/workflows/"+ns, qv, &l); err != nil {
					return nil, fmt.Errorf("argo: workflows in %s: %w", ns, err)
				}
				for _, w := range l.Items {
					f := a.workflowFacet(w, labels, server)
					if listed[f.SourceID] || !q.Since.IsZero() && f.StartedAt.Before(q.Since) {
						continue
					}
					listed[f.SourceID] = true
					out = append(out, f)
				}
				if l.Metadata.Continue == "" {
					break
				}
				cont = l.Metadata.Continue
			}
			if archived {
				var l list
				if err := c.GetJSON(ctx, "/api/v1/archived-workflows", map[string][]string{"namespace": {ns}, "listOptions.limit": {"500"}}, &l); err == nil {
					for _, w := range l.Items {
						f := a.workflowFacet(w, labels, server)
						if listed[f.SourceID] || !q.Since.IsZero() && f.StartedAt.Before(q.Since) {
							continue
						}
						listed[f.SourceID] = true
						out = append(out, f)
					}
				}
			}
		}
		if q.Wants(adapter.KindPipeline) {
			var tpl struct {
				Items []struct {
					Metadata struct {
						Name    string `json:"name"`
						UID     string `json:"uid"`
						Created string `json:"creationTimestamp"`
					} `json:"metadata"`
				} `json:"items"`
			}
			if err := c.GetJSON(ctx, "/api/v1/workflow-templates/"+ns, nil, &tpl); err == nil {
				for _, t := range tpl.Items {
					out = append(out, adapter.Facet{
						Source: "argo", SourceID: "template:" + ns + "/" + t.Metadata.Name, Kind: adapter.KindPipeline,
						Title: t.Metadata.Name, Status: adapter.Succeeded, StartedAt: orNow(httpx.ParseTime(t.Metadata.Created)),
						Keys:   map[string]string{"argo.template": ns + "/" + t.Metadata.Name},
						Fields: map[string]any{"namespace": ns, "type": "WorkflowTemplate"},
						Links:  []adapter.Link{{Label: "Open in Argo", URL: server + "/workflow-templates/" + ns + "/" + t.Metadata.Name}},
					})
				}
			}
			var cron struct {
				Items []struct {
					Metadata struct {
						Name    string `json:"name"`
						Created string `json:"creationTimestamp"`
					} `json:"metadata"`
					Spec struct {
						Schedule string `json:"schedule"`
						Suspend  bool   `json:"suspend"`
					} `json:"spec"`
					Status struct {
						LastScheduledTime string `json:"lastScheduledTime"`
						Active            []any  `json:"active"`
					} `json:"status"`
				} `json:"items"`
			}
			if err := c.GetJSON(ctx, "/api/v1/cron-workflows/"+ns, nil, &cron); err == nil {
				for _, cw := range cron.Items {
					st := adapter.Succeeded
					if cw.Spec.Suspend {
						st = adapter.Pending
					}
					out = append(out, adapter.Facet{
						Source: "argo", SourceID: "cron:" + ns + "/" + cw.Metadata.Name, Kind: adapter.KindPipeline,
						Title: cw.Metadata.Name, Status: st, StartedAt: orNow(httpx.ParseTime(cw.Metadata.Created)),
						Keys:   map[string]string{"argo.cron": ns + "/" + cw.Metadata.Name},
						Fields: map[string]any{"namespace": ns, "type": "CronWorkflow", "schedule": cw.Spec.Schedule, "suspended": cw.Spec.Suspend, "last_scheduled": cw.Status.LastScheduledTime, "active": len(cw.Status.Active)},
						Links:  []adapter.Link{{Label: "Open in Argo", URL: server + "/cron-workflows/" + ns + "/" + cw.Metadata.Name}},
					})
				}
			}
		}
	}
	return out, nil
}

func (a *Argo) workflowFacet(w workflow, labels map[string]string, server string) adapter.Facet {
	ns, name := w.Metadata.Namespace, w.Metadata.Name
	keys := map[string]string{adapter.KeyArgoWorkflow: name}
	if w.Metadata.UID != "" {
		keys[adapter.KeyArgoUID] = w.Metadata.UID
	}
	if v := w.Metadata.Labels["pipeline/runid"]; v != "" {
		keys[adapter.KeyKFPRunID] = v
	}
	if v := firstNonEmpty(w.Metadata.Labels["metaflow/run_id"], w.Metadata.Annotations["metaflow/run_id"], w.Metadata.Labels["metaflow.org/run-id"]); v != "" {
		if flow := firstNonEmpty(w.Metadata.Labels["metaflow/flow_name"], w.Metadata.Annotations["metaflow/flow_name"]); flow != "" {
			v = flow + "/" + v
		}
		keys[adapter.KeyMetaflowRunID] = v
	}
	if v := firstNonEmpty(w.Metadata.Annotations["fendlemark.io/git-sha"], w.Metadata.Labels["app.kubernetes.io/version"]); looksLikeSHA(v) {
		keys[adapter.KeyGitSHA] = v
	}
	if v := w.Metadata.Annotations["fendlemark.io/git-repo"]; v != "" {
		keys[adapter.KeyGitRepo] = v
	}
	params := map[string]string{}
	for _, p := range w.Spec.Arguments.Parameters {
		params[p.Name] = p.Value
	}
	type node struct {
		Name, DisplayName, Type, Phase, Template, StartedAt, FinishedAt string
	}
	var nodes []node
	for _, n := range w.Status.Nodes {
		nodes = append(nodes, node{n.Name, n.DisplayName, n.Type, n.Phase, n.TemplateName, n.StartedAt, n.FinishedAt})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].StartedAt < nodes[j].StartedAt })
	nodeFields := make([]map[string]string, 0, len(nodes))
	for _, n := range nodes {
		nodeFields = append(nodeFields, map[string]string{"name": n.DisplayName, "id": n.Name, "type": n.Type, "phase": n.Phase, "template": n.Template, "started_at": n.StartedAt, "ended_at": n.FinishedAt})
	}
	fields := map[string]any{"namespace": ns, "entrypoint": w.Spec.Entrypoint, "template_ref": w.Spec.WorkflowTemplateRef.Name, "progress": w.Status.Progress, "message": w.Status.Message, "parameters": params, "labels": w.Metadata.Labels, "nodes": nodeFields}
	for k, v := range labels {
		fields["label."+k] = v
	}
	started := httpx.ParseTime(w.Status.StartedAt)
	if started.IsZero() {
		started = orNow(httpx.ParseTime(w.Metadata.Created))
	}
	return adapter.Facet{
		Source: "argo", SourceID: ns + "/" + name, Kind: adapter.KindWorkflow, Title: name,
		Status: phase(w.Status.Phase, w.Metadata.Labels), StartedAt: started, EndedAt: httpx.ParseTime(w.Status.FinishedAt),
		Keys: keys, Fields: fields,
		Links: []adapter.Link{{Label: "Open in Argo", URL: server + "/workflows/" + ns + "/" + name}},
	}
}

func phase(p string, labels map[string]string) adapter.Phase {
	if p == "" {
		p = labels["workflows.argoproj.io/phase"] // briefly absent after creation on newer Argo
	}
	switch p {
	case "Pending":
		return adapter.Pending
	case "Running":
		return adapter.Running
	case "Succeeded":
		return adapter.Succeeded
	case "Failed", "Error":
		return adapter.Failed
	}
	return adapter.Unknown
}

// Watch polls the list endpoint and emits an event for every workflow whose
// phase, finish time, or node set changed. Argo Server's streaming endpoint
// uses a framing this build does not consume; polling at poll_interval is
// indistinguishable at the lattice's reconcile interval.
func (a *Argo) Watch(ctx context.Context) (<-chan adapter.Event, error) {
	a.mu.RLock()
	c, every := a.c, a.pollEvery
	a.mu.RUnlock()
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
			facets, err := a.Facets(ctx, adapter.Query{Kinds: []adapter.Kind{adapter.KindWorkflow}})
			if err == nil {
				now := map[string]bool{}
				for _, f := range facets {
					now[f.Ref()] = true
					sig := string(f.Status) + "|" + f.EndedAt.Format(time.RFC3339) + "|" + fmt.Sprint(len(f.Fields["nodes"].([]map[string]string)))
					if seen[f.Ref()] != sig {
						seen[f.Ref()] = sig
						select {
						case ch <- adapter.Event{Op: adapter.Upsert, Facet: f}:
						case <-ctx.Done():
							return
						}
					}
				}
				for ref := range seen {
					if !now[ref] {
						delete(seen, ref)
						src, id, _ := strings.Cut(ref, "\x00")
						select {
						case ch <- adapter.Event{Op: adapter.Delete, Facet: adapter.Facet{Source: src, SourceID: id, Kind: adapter.KindWorkflow, StartedAt: time.Now()}}:
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

func (a *Argo) Health(ctx context.Context) adapter.Status {
	a.mu.RLock()
	c, nss := a.c, a.namespaces
	a.mu.RUnlock()
	if c == nil {
		return adapter.Status{State: adapter.Disabled, Detail: "not connected"}
	}
	start := time.Now()
	var info map[string]any
	if err := c.GetJSON(ctx, "/api/v1/info", nil, &info); err != nil {
		return adapter.Status{State: adapter.Unreachable, Detail: err.Error(), Latency: time.Since(start)}
	}
	return adapter.Status{State: adapter.Healthy, Detail: fmt.Sprintf("%d namespaces", len(nss)), Latency: time.Since(start)}
}

func looksLikeSHA(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
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
