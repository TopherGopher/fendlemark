<script setup lang="ts">
import { computed } from 'vue'
import type { Entity } from '../types'
import { duration, phaseToken } from '../lattice'

const props = withDefaults(defineProps<{ entity: Entity; zoom?: number }>(), { zoom: 1 })

interface Span { label: string; source: string; start: number; end: number; phase: string }

/** Every facet (and any step-like child it reports) on one time axis. */
const spans = computed<Span[]>(() => {
  const out: Span[] = []
  for (const f of props.entity.facets) {
    const start = Date.parse(f.started_at)
    const end = f.ended_at ? Date.parse(f.ended_at) : Date.now()
    out.push({ label: f.title, source: f.source, start, end, phase: f.status })
    const children = (f.fields?.nodes ?? f.fields?.steps ?? f.fields?.tasks) as Array<Record<string, string>> | undefined
    for (const c of children ?? []) {
      const cs = Date.parse(c.started_at ?? c.start ?? ''), ce = c.ended_at ?? c.end ? Date.parse(c.ended_at ?? c.end) : end
      if (isFinite(cs)) out.push({ label: c.name ?? c.id ?? '', source: f.source, start: cs, end: ce, phase: c.phase ?? c.status ?? 'Unknown' })
    }
  }
  return out.sort((a, b) => a.start - b.start)
})
const t0 = computed(() => Math.min(...spans.value.map((s) => s.start)))
const t1 = computed(() => Math.max(...spans.value.map((s) => s.end)))
const pct = (t: number) => ((t - t0.value) / Math.max(1, t1.value - t0.value)) * 100 * props.zoom
</script>

<template>
  <div class="fm-timeline" role="img" :aria-label="`Timeline of ${entity.title}`">
    <div v-for="(s, i) in spans" :key="i" class="fm-timeline__row">
      <span class="fm-timeline__label"><span class="fm-timeline__src">{{ s.source }}</span>{{ s.label }}</span>
      <div class="fm-timeline__track">
        <div class="fm-timeline__bar" :style="{ left: pct(s.start) + '%', width: Math.max(0.5, pct(s.end) - pct(s.start)) + '%', background: phaseToken(s.phase) }" :title="`${s.label}: ${duration(new Date(s.start).toISOString(), new Date(s.end).toISOString())}`" />
      </div>
    </div>
  </div>
</template>

<style>
.fm-timeline { font-family: var(--fm-font-sans); font-size: .82rem; display: grid; gap: .3rem; overflow-x: auto; }
.fm-timeline__row { display: grid; grid-template-columns: minmax(10rem, 16rem) 1fr; gap: .75rem; align-items: center; }
.fm-timeline__label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--fm-fg); }
.fm-timeline__src { font: 600 .68rem var(--fm-font-mono); color: var(--fm-accent); margin-right: .4rem; }
.fm-timeline__track { position: relative; height: .9rem; background: var(--fm-bg-sunken); border-radius: 4px; }
.fm-timeline__bar { position: absolute; top: 0; height: 100%; border-radius: 4px; min-width: 2px; }
</style>
