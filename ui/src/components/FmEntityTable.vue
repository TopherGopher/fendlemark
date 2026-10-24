<script setup lang="ts">
import { computed, inject, ref, watch } from 'vue'
import type { Entity, EntityQuery, Kind } from '../types'
import { LATTICE_KEY } from '../keys'
import FmStatusBadge from './FmStatusBadge.vue'
import FmEmptyState from './FmEmptyState.vue'
import { duration } from '../lattice'

const props = withDefaults(defineProps<{ kind: Kind; query?: EntityQuery; pageSize?: number; serenity?: boolean }>(), { pageSize: 50, serenity: false })
const emit = defineEmits<{ select: [entity: Entity] }>()
const lattice = inject(LATTICE_KEY, null)
const page = ref(0)
const all = computed(() => lattice ? lattice.ofKind(props.kind).value : [])
const rows = computed(() => all.value.slice(page.value * props.pageSize, (page.value + 1) * props.pageSize))
const pages = computed(() => Math.max(1, Math.ceil(all.value.length / props.pageSize)))
watch(() => props.kind, () => (page.value = 0))
const sources = (e: Entity) => [...new Set(e.facets.map((f) => f.source))].sort()
</script>

<template>
  <div class="fm-table">
    <FmEmptyState v-if="lattice && lattice.ready.value && !all.length" :heading="`No ${kind}s yet.`" hint="When an enabled adapter reports one, it appears here." />
    <FmEmptyState v-else-if="!lattice" heading="No lattice provided." hint="Call provideLattice(integrateEasy(...)) in an ancestor component." />
    <table v-else>
      <thead><tr><th>Title</th><th>Status</th><th>Sources</th><th>Started</th><th>Duration</th></tr></thead>
      <tbody>
        <tr v-for="e in rows" :key="e.id" tabindex="0" @click="emit('select', e)" @keydown.enter="emit('select', e)">
          <td class="fm-table__title"><slot name="title" :entity="e">{{ e.title }}</slot></td>
          <td><slot name="status" :entity="e"><FmStatusBadge :phase="e.status" size="sm" :serenity="serenity" /></slot></td>
          <td><span v-for="s in sources(e)" :key="s" class="fm-chip">{{ s }}</span></td>
          <td class="fm-num">{{ new Date(e.started_at).toLocaleString() }}</td>
          <td class="fm-num">{{ duration(e.started_at, e.ended_at) }}</td>
        </tr>
      </tbody>
    </table>
    <nav v-if="pages > 1" class="fm-table__pager" aria-label="Pages">
      <button type="button" :disabled="page === 0" @click="page--">Previous</button>
      <span>{{ page + 1 }} / {{ pages }}</span>
      <button type="button" :disabled="page >= pages - 1" @click="page++">Next</button>
    </nav>
  </div>
</template>

<style>
.fm-table { font-family: var(--fm-font-sans); overflow-x: auto; }
.fm-table table { border-collapse: collapse; width: 100%; font-size: .88rem; }
.fm-table th { text-align: left; font-size: .72rem; letter-spacing: .05em; text-transform: uppercase; color: var(--fm-fg-muted); padding: .5rem .7rem; border-bottom: 1px solid var(--fm-border); background: var(--fm-bg-sunken); }
.fm-table td { padding: .55rem .7rem; border-bottom: 1px solid var(--fm-border); vertical-align: middle; }
.fm-table tbody tr { cursor: pointer; }
.fm-table tbody tr:hover, .fm-table tbody tr:focus-visible { background: var(--fm-accent-soft); outline: none; }
.fm-table__title { font-weight: 500; max-width: 32rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fm-num { font-variant-numeric: tabular-nums; color: var(--fm-fg-muted); white-space: nowrap; }
.fm-chip { display: inline-block; font: 600 .68rem var(--fm-font-mono); color: var(--fm-accent); background: var(--fm-accent-soft); border-radius: 999px; padding: .1rem .45rem; margin-right: .25rem; }
.fm-table__pager { display: flex; gap: 1rem; align-items: center; justify-content: flex-end; padding: .6rem 0; font-size: .85rem; color: var(--fm-fg-muted); }
.fm-table__pager button { font: inherit; background: var(--fm-bg-elev); border: 1px solid var(--fm-border); border-radius: 6px; padding: .3rem .7rem; color: var(--fm-fg); cursor: pointer; }
.fm-table__pager button:disabled { opacity: .5; cursor: default; }
</style>
