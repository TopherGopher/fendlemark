import { computed, reactive, readonly, ref, shallowRef, type ComputedRef, type Ref } from 'vue'
import { LatticeClient, type ClientOptions } from './client'
import type { AdapterStatus, Delta, Entity, EntityQuery, Kind, LatticeStats } from './types'

export interface IntegrateOptions extends ClientOptions {
  /** Kinds to hold in the client-side store. Default: everything the server has. */
  kinds?: Kind[]
  /** Maximum entities to hold client-side. Oldest by started_at are evicted. Default 5000. */
  maxEntities?: number
  /** Keep the SSE subscription open. Default true in browsers. */
  live?: boolean
}

export interface Lattice {
  readonly client: LatticeClient
  /** Entities by lattice id, reactive. */
  readonly entities: Readonly<Record<string, Entity>>
  /** Adapter health, reactive. */
  readonly adapters: Ref<AdapterStatus[]>
  readonly stats: Ref<LatticeStats | null>
  readonly generation: Ref<number>
  readonly ready: Ref<boolean>
  readonly error: Ref<Error | null>
  /** Entities of a kind, newest first, as a computed list. */
  ofKind(kind: Kind): ComputedRef<Entity[]>
  /** Fetch (or refetch) one entity into the store. */
  load(id: string): Promise<Entity>
  /** Run a server-side query and merge the results into the store. */
  query(q: EntityQuery): Promise<Entity[]>
  refresh(): Promise<void>
  stop(): void
}

const KIND_ORDER: Kind[] = ['run', 'workflow', 'pipeline', 'model', 'artifact', 'notebook']

/**
 * integrateEasy connects to a Fendlemark control plane and returns a reactive
 * lattice: every entity the server knows about, adapter health, and the
 * generation number, kept current over server-sent events.
 *
 * It is the whole integration. One call, one object, every tool.
 */
export function integrateEasy(opts: IntegrateOptions = {}): Lattice {
  const client = new LatticeClient(opts)
  const entities = reactive<Record<string, Entity>>({})
  const adapters = ref<AdapterStatus[]>([])
  const stats = shallowRef<LatticeStats | null>(null)
  const generation = ref(0)
  const ready = ref(false)
  const error = ref<Error | null>(null)
  const max = opts.maxEntities ?? 5000
  const kinds = opts.kinds ?? KIND_ORDER

  const merge = (list: Entity[]) => {
    for (const e of list) entities[e.id] = e
    const ids = Object.keys(entities)
    if (ids.length > max) {
      ids.sort((a, b) => (entities[a].started_at < entities[b].started_at ? -1 : 1))
      for (const id of ids.slice(0, ids.length - max)) delete entities[id]
    }
  }

  const refresh = async () => {
    try {
      const [list, ad, st] = await Promise.all([
        client.entities({ kind: kinds, limit: max }),
        client.adapters(),
        client.stats(),
      ])
      for (const id of Object.keys(entities)) delete entities[id]
      merge(list.entities)
      adapters.value = ad.adapters
      stats.value = st
      generation.value = st.generation
      error.value = null
      ready.value = true
    } catch (e) {
      error.value = e as Error
      ready.value = true
    }
  }

  const onDelta = async (d: Delta) => {
    // A gap in generations means we missed events; rebuild rather than guess.
    if (generation.value && d.generation > generation.value + 1) return refresh()
    for (const id of d.removed) delete entities[id]
    const fetched = await Promise.all(d.upserted.map((id) => client.entity(id).catch(() => null)))
    merge(fetched.filter((e): e is Entity => !!e && kinds.includes(e.kind)))
    generation.value = d.generation
  }

  const stopEvents = opts.live === false ? () => {} : client.events(onDelta)
  void refresh()

  const byKind = new Map<Kind, ComputedRef<Entity[]>>()
  const ofKind = (kind: Kind) => {
    let c = byKind.get(kind)
    if (!c) {
      c = computed(() => Object.values(entities).filter((e) => e.kind === kind).sort((a, b) => (a.started_at < b.started_at ? 1 : -1)))
      byKind.set(kind, c)
    }
    return c
  }

  return {
    client,
    entities: readonly(entities) as Readonly<Record<string, Entity>>,
    adapters,
    stats,
    generation,
    ready,
    error,
    ofKind,
    async load(id) { const e = await client.entity(id); merge([e]); return e },
    async query(q) { const r = await client.entities(q); merge(r.entities); return r.entities },
    refresh,
    stop: stopEvents,
  }
}

/** Phase colour token name for a phase. Shared by every adapter's panel. */
export function phaseToken(p: string): string {
  switch (p) {
    case 'Running': return 'var(--fm-phase-running)'
    case 'Succeeded': return 'var(--fm-phase-succeeded)'
    case 'Failed': return 'var(--fm-phase-failed)'
    case 'Pending': return 'var(--fm-phase-pending)'
    default: return 'var(--fm-fg-muted)'
  }
}

/** Human duration between two ISO timestamps, or since start if no end. */
export function duration(start: string, end?: string): string {
  const ms = (end ? Date.parse(end) : Date.now()) - Date.parse(start)
  if (!isFinite(ms) || ms < 0) return ''
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s`
  if (s < 3600) return `${Math.floor(s / 60)}m ${s % 60}s`
  if (s < 86400) return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`
  return `${Math.floor(s / 86400)}d ${Math.floor((s % 86400) / 3600)}h`
}
