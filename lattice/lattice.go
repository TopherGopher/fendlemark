// Package lattice is Fendlemark's in-memory entity graph. It exists because
// the same real-world thing shows up in several tools under several names.
//
// Reconciliation is deterministic and rule-based: facets that share an exact
// strong correlation key within the correlation window are merged into one
// entity, weak keys create related-to edges, and two lattices built from the
// same facets produce identical entities with identical ids.
package lattice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"fendlemark.develops.guru/fendlemark/adapter"
)

// ID is a stable entity identifier: "lt_" followed by 16 hex characters derived
// from the sorted set of facet refs that merged into the entity.
type ID string

// RelKind is the kind of edge between two entities.
type RelKind string

const (
	ProducedBy   RelKind = "ProducedBy"
	DerivedFrom  RelKind = "DerivedFrom"
	DeployedAs   RelKind = "DeployedAs"
	SharesCommit RelKind = "SharesCommit"
)

// Relation is a directed edge to another entity.
type Relation struct {
	To  ID      `json:"to"`
	Via string  `json:"via"`
	Rel RelKind `json:"rel"`
}

// Entity is the thing the console shows.
type Entity struct {
	ID        ID              `json:"id"`
	Kind      adapter.Kind    `json:"kind"`
	Title     string          `json:"title"`
	Status    adapter.Phase   `json:"status"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   time.Time       `json:"ended_at,omitempty"`
	Facets    []adapter.Facet `json:"facets"`
	Related   []Relation      `json:"related,omitempty"`
}

// Query selects entities.
type Query struct {
	Kinds  []adapter.Kind
	Keys   map[string]string
	Status []adapter.Phase
	Since  time.Time
	Until  time.Time
	Limit  int
	Offset int
}

// Delta is what changed in one reconciliation.
type Delta struct {
	Upserted   []ID   `json:"upserted"`
	Removed    []ID   `json:"removed"`
	Generation uint64 `json:"generation"`
}

// MergeStep records why two facets became one entity.
type MergeStep struct {
	A     string        `json:"a"`
	B     string        `json:"b"`
	Key   string        `json:"key"`
	Value string        `json:"value"`
	Gap   time.Duration `json:"gap"`
}

// RelationStep records why an edge exists.
type RelationStep struct {
	From ID      `json:"from"`
	To   ID      `json:"to"`
	Key  string  `json:"key"`
	Rel  RelKind `json:"rel"`
}

// Explanation is the output of Explain.
type Explanation struct {
	Entity    Entity         `json:"entity"`
	Merges    []MergeStep    `json:"merges"`
	Relations []RelationStep `json:"relations"`
}

// KeyDef declares an additional correlation key.
type KeyDef struct {
	Name string
	Weak bool
}

// Options configures a Lattice.
type Options struct {
	CorrelationWindow time.Duration // default 90s
	NoWeakKeys        bool          // weak keys (git.sha, git.repo) create related-to edges unless disabled
	CustomKeys        []KeyDef
	RetainOrphans     time.Duration // default 1h
	MaxEntities       int           // default 200000
	Now               func() time.Time
}

// Stats summarises the lattice.
type Stats struct {
	Generation   uint64         `json:"generation"`
	Entities     int            `json:"entities"`
	Facets       int            `json:"facets"`
	ByKind       map[string]int `json:"by_kind"`
	BySource     map[string]int `json:"by_source"`
	Correlations map[string]int `json:"correlations"`
}

type stored struct {
	facet adapter.Facet
	seen  time.Time
}

// Lattice holds facets and the entities reconciled from them.
type Lattice struct {
	mu        sync.RWMutex
	opts      Options
	weak      map[string]bool
	facets    map[string]stored // by facet ref
	entities  map[ID]*Entity
	owner     map[string]ID // facet ref -> entity
	merges    map[ID][]MergeStep
	relsteps  map[ID][]RelationStep
	prints    map[ID]string
	keyCounts map[string]int
	gen       uint64
	subs      map[chan Delta]struct{}
}

// New constructs an empty lattice.
func New(opts Options) *Lattice {
	if opts.CorrelationWindow <= 0 {
		opts.CorrelationWindow = 90 * time.Second
	}
	if opts.RetainOrphans <= 0 {
		opts.RetainOrphans = time.Hour
	}
	if opts.MaxEntities <= 0 {
		opts.MaxEntities = 200000
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	l := &Lattice{
		opts:      opts,
		weak:      map[string]bool{adapter.KeyGitSHA: true, adapter.KeyGitRepo: true},
		facets:    map[string]stored{},
		entities:  map[ID]*Entity{},
		owner:     map[string]ID{},
		merges:    map[ID][]MergeStep{},
		relsteps:  map[ID][]RelationStep{},
		prints:    map[ID]string{},
		keyCounts: map[string]int{},
		subs:      map[chan Delta]struct{}{},
	}
	for _, k := range opts.CustomKeys {
		if k.Weak {
			l.weak[k.Name] = true
		}
	}
	return l
}

// Apply replaces the facets of every source present in the batch, retains
// orphans for RetainOrphans, and reconciles. It is the full reconciliation.
func (l *Lattice) Apply(facets []adapter.Facet) Delta {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.opts.Now()
	present := map[string]bool{}
	incoming := map[string]bool{}
	for _, f := range facets {
		if err := f.Validate(); err != nil {
			continue
		}
		present[f.Source] = true
		incoming[f.Ref()] = true
		l.facets[f.Ref()] = stored{facet: f, seen: now}
	}
	for ref, s := range l.facets {
		if present[s.facet.Source] && !incoming[ref] && now.Sub(s.seen) > l.opts.RetainOrphans {
			delete(l.facets, ref)
		}
	}
	return l.reconcile()
}

// ApplyEvent applies one incremental change and reconciles.
func (l *Lattice) ApplyEvent(ev adapter.Event) Delta {
	l.mu.Lock()
	defer l.mu.Unlock()
	ref := ev.Facet.Ref()
	switch ev.Op {
	case adapter.Delete:
		delete(l.facets, ref)
	default:
		if err := ev.Facet.Validate(); err != nil {
			return Delta{Generation: l.gen}
		}
		l.facets[ref] = stored{facet: ev.Facet, seen: l.opts.Now()}
	}
	return l.reconcile()
}

type dsu struct{ parent []int }

func newDSU(n int) *dsu {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &dsu{parent: p}
}
func (d *dsu) find(i int) int {
	for d.parent[i] != i {
		d.parent[i] = d.parent[d.parent[i]]
		i = d.parent[i]
	}
	return i
}
func (d *dsu) union(a, b int) bool {
	ra, rb := d.find(a), d.find(b)
	if ra == rb {
		return false
	}
	if ra > rb { // smaller index roots, for determinism
		ra, rb = rb, ra
	}
	d.parent[rb] = ra
	return true
}

var kindPrecedence = map[adapter.Kind]int{
	adapter.KindRun: 0, adapter.KindWorkflow: 1, adapter.KindPipeline: 2,
	adapter.KindModel: 3, adapter.KindArtifact: 4, adapter.KindNotebook: 5,
}
var sourcePrecedence = map[string]int{"mlflow": 0, "metaflow": 1, "argo": 2, "kubeflow": 3}

func sourceRank(s string) int {
	if r, ok := sourcePrecedence[s]; ok {
		return r
	}
	return 10
}

// reconcile is a pure function of l.facets. Caller holds the write lock.
func (l *Lattice) reconcile() Delta {
	// Enforce the entity cap by dropping the oldest facets.
	if len(l.facets) > l.opts.MaxEntities {
		all := make([]stored, 0, len(l.facets))
		for _, s := range l.facets {
			all = append(all, s)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].facet.StartedAt.Before(all[j].facet.StartedAt) })
		for _, s := range all[:len(all)-l.opts.MaxEntities] {
			delete(l.facets, s.facet.Ref())
		}
	}

	refs := make([]string, 0, len(l.facets))
	for ref := range l.facets {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	idx := make(map[string]int, len(refs))
	fs := make([]adapter.Facet, len(refs))
	for i, ref := range refs {
		idx[ref] = i
		fs[i] = l.facets[ref].facet
	}

	// Strong-key linking: for each (key, value), sort members by StartedAt and
	// link neighbours within the window. Deterministic and O(n log n).
	type member struct {
		i  int
		at time.Time
	}
	groups := map[string][]member{}
	for i, f := range fs {
		for k, v := range f.Keys {
			if l.weak[k] {
				continue
			}
			g := k + "\x00" + v
			groups[g] = append(groups[g], member{i, f.StartedAt})
		}
	}
	gkeys := make([]string, 0, len(groups))
	for g := range groups {
		gkeys = append(gkeys, g)
	}
	sort.Strings(gkeys)
	d := newDSU(len(fs))
	mergesByRoot := map[int][]MergeStep{}
	keyCounts := map[string]int{}
	for _, g := range gkeys {
		ms := groups[g]
		sort.Slice(ms, func(a, b int) bool {
			if !ms[a].at.Equal(ms[b].at) {
				return ms[a].at.Before(ms[b].at)
			}
			return ms[a].i < ms[b].i
		})
		k, v, _ := strings.Cut(g, "\x00")
		for j := 1; j < len(ms); j++ {
			gap := ms[j].at.Sub(ms[j-1].at)
			if gap > l.opts.CorrelationWindow {
				continue
			}
			if fs[ms[j].i].Source == fs[ms[j-1].i].Source {
				// Same adapter, same key: two distinct upstream objects. Never merge.
				continue
			}
			if d.union(ms[j-1].i, ms[j].i) {
				keyCounts[k]++
				root := d.find(ms[j].i)
				mergesByRoot[root] = append(mergesByRoot[root], MergeStep{A: refs[ms[j-1].i], B: refs[ms[j].i], Key: k, Value: v, Gap: gap})
			}
		}
	}

	// Components -> entities.
	comps := map[int][]int{}
	for i := range fs {
		r := d.find(i)
		comps[r] = append(comps[r], i)
	}
	newEntities := map[ID]*Entity{}
	newOwner := map[string]ID{}
	newMerges := map[ID][]MergeStep{}
	byKey := map[string]ID{} // "key\x00value" -> entity, for weak/derived relations
	for root, members := range comps {
		sort.Ints(members)
		ent := &Entity{}
		h := sha256.New()
		for _, i := range members {
			h.Write([]byte(refs[i]))
			h.Write([]byte{0})
			ent.Facets = append(ent.Facets, fs[i])
		}
		ent.ID = ID("lt_" + hex.EncodeToString(h.Sum(nil))[:16])
		sort.Slice(ent.Facets, func(a, b int) bool {
			ra, rb := sourceRank(ent.Facets[a].Source), sourceRank(ent.Facets[b].Source)
			if ra != rb {
				return ra < rb
			}
			return ent.Facets[a].Ref() < ent.Facets[b].Ref()
		})
		ent.Kind = ent.Facets[0].Kind
		ent.Title = ent.Facets[0].Title
		ent.Status = adapter.Unknown
		var anyFailed, anyRunning, anyPending, allSucceeded bool
		allSucceeded = true
		for _, f := range ent.Facets {
			if kindPrecedence[f.Kind] < kindPrecedence[ent.Kind] {
				ent.Kind = f.Kind
			}
			if ent.StartedAt.IsZero() || f.StartedAt.Before(ent.StartedAt) {
				ent.StartedAt = f.StartedAt
			}
			if f.EndedAt.After(ent.EndedAt) {
				ent.EndedAt = f.EndedAt
			}
			switch f.Status {
			case adapter.Failed:
				anyFailed = true
			case adapter.Running:
				anyRunning = true
			case adapter.Pending:
				anyPending = true
			}
			if f.Status != adapter.Succeeded {
				allSucceeded = false
			}
			for k, v := range f.Keys {
				byKey[k+"\x00"+v] = ent.ID
			}
			newOwner[f.Ref()] = ent.ID
		}
		switch {
		case anyFailed:
			ent.Status = adapter.Failed
		case anyRunning:
			ent.Status = adapter.Running
			ent.EndedAt = time.Time{}
		case anyPending:
			ent.Status = adapter.Pending
		case allSucceeded:
			ent.Status = adapter.Succeeded
		}
		newEntities[ent.ID] = ent
		newMerges[ent.ID] = mergesByRoot[root]
	}

	// Relations: weak keys (git.sha + git.repo) and model -> source run.
	newRel := map[ID][]RelationStep{}
	addRel := func(from, to ID, key string, rel RelKind) {
		if from == to {
			return
		}
		e := newEntities[from]
		for _, r := range e.Related {
			if r.To == to && r.Rel == rel {
				return
			}
		}
		e.Related = append(e.Related, Relation{To: to, Via: key, Rel: rel})
		newRel[from] = append(newRel[from], RelationStep{From: from, To: to, Key: key, Rel: rel})
	}
	if !l.opts.NoWeakKeys {
		commits := map[string][]ID{}
		for _, e := range newEntities {
			seen := map[string]bool{}
			for _, f := range e.Facets {
				sha, repo := f.Keys[adapter.KeyGitSHA], f.Keys[adapter.KeyGitRepo]
				if sha == "" || repo == "" {
					continue
				}
				g := repo + "@" + sha
				if !seen[g] {
					seen[g] = true
					commits[g] = append(commits[g], e.ID)
				}
			}
		}
		for g, ids := range commits {
			sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
			if len(ids) > 50 {
				ids = ids[:50]
			}
			for a := range ids {
				for b := range ids {
					if a != b {
						addRel(ids[a], ids[b], g, SharesCommit)
					}
				}
			}
		}
	}
	for _, e := range newEntities {
		for _, f := range e.Facets {
			if f.Kind != adapter.KindModel {
				continue
			}
			if src, ok := f.Fields["source_run_id"].(string); ok && src != "" {
				if run, ok := byKey[adapter.KeyMLflowRunID+"\x00"+src]; ok {
					addRel(e.ID, run, adapter.KeyMLflowRunID, DerivedFrom)
					addRel(run, e.ID, adapter.KeyMLflowRunID, DeployedAs)
				}
			}
		}
	}
	for _, e := range newEntities {
		sort.Slice(e.Related, func(a, b int) bool {
			if e.Related[a].Rel != e.Related[b].Rel {
				return e.Related[a].Rel < e.Related[b].Rel
			}
			return e.Related[a].To < e.Related[b].To
		})
	}

	// Delta against the previous generation.
	var delta Delta
	newPrints := map[ID]string{}
	for id, e := range newEntities {
		p := fingerprint(e)
		newPrints[id] = p
		if old, ok := l.prints[id]; !ok || old != p {
			delta.Upserted = append(delta.Upserted, id)
		}
	}
	for id := range l.entities {
		if _, ok := newEntities[id]; !ok {
			delta.Removed = append(delta.Removed, id)
		}
	}
	sort.Slice(delta.Upserted, func(a, b int) bool { return delta.Upserted[a] < delta.Upserted[b] })
	sort.Slice(delta.Removed, func(a, b int) bool { return delta.Removed[a] < delta.Removed[b] })

	l.entities, l.owner, l.merges, l.relsteps, l.prints, l.keyCounts = newEntities, newOwner, newMerges, newRel, newPrints, keyCounts
	if len(delta.Upserted) > 0 || len(delta.Removed) > 0 || l.gen == 0 {
		l.gen++
	}
	delta.Generation = l.gen
	for ch := range l.subs {
		select {
		case ch <- delta:
		default: // slow subscriber; it will notice the generation gap and refresh
		}
	}
	return delta
}

func fingerprint(e *Entity) string {
	var b strings.Builder
	b.WriteString(string(e.Kind))
	b.WriteByte('|')
	b.WriteString(string(e.Status))
	b.WriteByte('|')
	b.WriteString(e.Title)
	b.WriteByte('|')
	b.WriteString(e.EndedAt.UTC().Format(time.RFC3339Nano))
	for _, f := range e.Facets {
		b.WriteByte('|')
		b.WriteString(f.Ref())
		b.WriteByte(':')
		b.WriteString(string(f.Status))
		b.WriteByte(':')
		b.WriteString(fmt.Sprint(len(f.Fields)))
	}
	for _, r := range e.Related {
		b.WriteByte('|')
		b.WriteString(string(r.To))
		b.WriteString(string(r.Rel))
	}
	return b.String()
}

// Get returns one entity.
func (l *Lattice) Get(id ID) (Entity, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.entities[id]
	if !ok {
		return Entity{}, false
	}
	return *e, true
}

// Query returns matching entities, newest first.
func (l *Lattice) Query(q Query) []Entity {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entity, 0, 64)
	for _, e := range l.entities {
		if !matches(e, q) {
			continue
		}
		out = append(out, *e)
	}
	sort.Slice(out, func(a, b int) bool {
		if !out[a].StartedAt.Equal(out[b].StartedAt) {
			return out[a].StartedAt.After(out[b].StartedAt)
		}
		return out[a].ID < out[b].ID
	})
	if q.Offset > 0 {
		if q.Offset >= len(out) {
			return nil
		}
		out = out[q.Offset:]
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out
}

// Count returns the number of entities matching q.
func (l *Lattice) Count(q Query) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	n := 0
	for _, e := range l.entities {
		if matches(e, q) {
			n++
		}
	}
	return n
}

func matches(e *Entity, q Query) bool {
	if len(q.Kinds) > 0 && !containsKind(q.Kinds, e.Kind) {
		return false
	}
	if len(q.Status) > 0 && !containsPhase(q.Status, e.Status) {
		return false
	}
	if !q.Since.IsZero() && e.StartedAt.Before(q.Since) {
		return false
	}
	if !q.Until.IsZero() && e.StartedAt.After(q.Until) {
		return false
	}
	for k, v := range q.Keys {
		found := false
		for _, f := range e.Facets {
			if f.Keys[k] == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsKind(ks []adapter.Kind, k adapter.Kind) bool {
	for _, kk := range ks {
		if kk == k {
			return true
		}
	}
	return false
}
func containsPhase(ps []adapter.Phase, p adapter.Phase) bool {
	for _, pp := range ps {
		if pp == p {
			return true
		}
	}
	return false
}

// Explain says why an entity has the facets and edges it has.
func (l *Lattice) Explain(id ID) (Explanation, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.entities[id]
	if !ok {
		return Explanation{}, false
	}
	return Explanation{Entity: *e, Merges: append([]MergeStep(nil), l.merges[id]...), Relations: append([]RelationStep(nil), l.relsteps[id]...)}, true
}

// Owner returns the entity holding a facet ref.
func (l *Lattice) Owner(ref string) (ID, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	id, ok := l.owner[ref]
	return id, ok
}

// Generation returns the reconciliation generation.
func (l *Lattice) Generation() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.gen
}

// Stats summarises the lattice.
func (l *Lattice) Stats() Stats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s := Stats{Generation: l.gen, Entities: len(l.entities), Facets: len(l.facets),
		ByKind: map[string]int{}, BySource: map[string]int{}, Correlations: map[string]int{}}
	for _, e := range l.entities {
		s.ByKind[string(e.Kind)]++
	}
	for _, f := range l.facets {
		s.BySource[f.facet.Source]++
	}
	for k, n := range l.keyCounts {
		s.Correlations[k] = n
	}
	return s
}

// Subscribe returns a channel of deltas until ctx is done. Slow subscribers
// miss deltas rather than block reconciliation; the generation number
// reveals the gap.
func (l *Lattice) Subscribe(ctx context.Context) <-chan Delta {
	ch := make(chan Delta, 16)
	l.mu.Lock()
	l.subs[ch] = struct{}{}
	l.mu.Unlock()
	go func() {
		<-ctx.Done()
		l.mu.Lock()
		delete(l.subs, ch)
		l.mu.Unlock()
		close(ch)
	}()
	return ch
}
