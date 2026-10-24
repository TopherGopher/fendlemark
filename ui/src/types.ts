/** Tool-agnostic lifecycle phase. Every adapter maps its own vocabulary onto these. */
export type Phase = 'Pending' | 'Running' | 'Succeeded' | 'Failed' | 'Unknown'
export type Kind = 'run' | 'workflow' | 'pipeline' | 'model' | 'artifact' | 'notebook'
export type AdapterState = 'healthy' | 'degraded' | 'unreachable' | 'disabled'

export interface Link { label: string; url: string }

/** One tool's view of one thing. Never merged; attached to an entity by the lattice. */
export interface Facet {
  source: string
  source_id: string
  kind: Kind
  title: string
  status: Phase
  started_at: string
  ended_at?: string
  keys?: Record<string, string>
  fields?: Record<string, unknown>
  links?: Link[]
}

export type RelKind = 'ProducedBy' | 'DerivedFrom' | 'DeployedAs' | 'SharesCommit'
export interface Relation { to: string; via: string; rel: RelKind }

/** The thing the console shows: a stable lattice id with one or more facets. */
export interface Entity {
  id: string
  kind: Kind
  title: string
  status: Phase
  started_at: string
  ended_at?: string
  facets: Facet[]
  related?: Relation[]
}

export interface AdapterStatus {
  name: string
  display_name: string
  state: AdapterState
  detail: string
  latency_ms: number
  facets: number
  maturity: string
}

export interface LatticeStats {
  generation: number
  entities: number
  facets: number
  by_kind: Record<string, number>
  by_source: Record<string, number>
  correlations: Record<string, number>
}

export interface Delta { upserted: string[]; removed: string[]; generation: number }

export interface EntityQuery {
  kind?: Kind | Kind[]
  status?: Phase | Phase[]
  key?: Record<string, string>
  since?: string
  until?: string
  limit?: number
  offset?: number
}
