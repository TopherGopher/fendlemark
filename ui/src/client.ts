import type { AdapterStatus, Delta, Entity, EntityQuery, LatticeStats } from './types'

export interface ClientOptions {
  /** Origin of a running Fendlemark control plane. Defaults to the page origin. */
  baseUrl?: string
  /** Extra headers, for deployments behind a proxy that wants them. */
  headers?: Record<string, string>
  /** fetch implementation, for tests and non-browser hosts. */
  fetch?: typeof fetch
}

export class FendlemarkError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) {
    super(message)
    this.name = 'FendlemarkError'
  }
}

/**
 * Thin, typed client over the control plane's `/api/v1`. The console never
 * talks to MLflow, Metaflow, Argo, or Kubeflow directly; everything goes
 * through the lattice, and so does this.
 */
export class LatticeClient {
  private readonly base: string
  private readonly headers: Record<string, string>
  private readonly f: typeof fetch

  constructor(opts: ClientOptions = {}) {
    this.base = (opts.baseUrl ?? (typeof location !== 'undefined' ? location.origin : '')).replace(/\/+$/, '')
    this.headers = { Accept: 'application/json', ...(opts.headers ?? {}) }
    this.f = opts.fetch ?? ((input, init) => fetch(input, init))
  }

  get baseUrl(): string { return this.base }

  private async get<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
    const u = new URL(this.base + path, this.base || 'http://localhost')
    for (const [k, v] of Object.entries(params ?? {})) if (v !== undefined && v !== '') u.searchParams.set(k, String(v))
    const res = await this.f(u.toString(), { headers: this.headers, credentials: 'same-origin' })
    if (!res.ok) {
      let code = `HTTP-${res.status}`, msg = res.statusText
      try { const body = await res.json(); code = body.code ?? code; msg = body.error ?? msg } catch { /* not json */ }
      throw new FendlemarkError(res.status, code, msg)
    }
    return (await res.json()) as T
  }

  entities(q: EntityQuery = {}): Promise<{ entities: Entity[]; total: number; generation: number }> {
    const list = (v?: string | string[]) => (Array.isArray(v) ? v.join(',') : v)
    const key = q.key ? Object.entries(q.key).map(([k, v]) => `${k}=${v}`).join(',') : undefined
    return this.get('/api/v1/entities', { kind: list(q.kind), status: list(q.status), key, since: q.since, until: q.until, limit: q.limit, offset: q.offset })
  }

  entity(id: string): Promise<Entity> { return this.get(`/api/v1/entities/${encodeURIComponent(id)}`) }
  explain(id: string): Promise<unknown> { return this.get(`/api/v1/entities/${encodeURIComponent(id)}/explain`) }
  adapters(): Promise<{ adapters: AdapterStatus[] }> { return this.get('/api/v1/adapters') }
  stats(): Promise<LatticeStats> { return this.get('/api/v1/lattice/stats') }

  /**
   * Subscribe to lattice deltas over server-sent events. Returns a stop function.
   * Reconnects with capped exponential backoff; the lattice generation number
   * lets the caller detect a gap and refetch.
   */
  events(onDelta: (d: Delta) => void, opts: { entity?: string; kind?: string; onError?: (e: Event) => void } = {}): () => void {
    if (typeof EventSource === 'undefined') return () => {}
    const u = new URL(this.base + '/api/v1/events', this.base || 'http://localhost')
    if (opts.entity) u.searchParams.set('entity', opts.entity)
    if (opts.kind) u.searchParams.set('kind', opts.kind)
    let es: EventSource | null = null
    let stopped = false
    let backoff = 500
    const open = () => {
      if (stopped) return
      es = new EventSource(u.toString(), { withCredentials: true })
      es.addEventListener('delta', (ev) => { backoff = 500; onDelta(JSON.parse((ev as MessageEvent).data)) })
      es.onerror = (e) => {
        opts.onError?.(e)
        es?.close()
        if (!stopped) setTimeout(open, backoff = Math.min(backoff * 2, 15000))
      }
    }
    open()
    return () => { stopped = true; es?.close() }
  }
}
