<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Facet } from '../types'
import FmDeepLink from './FmDeepLink.vue'
import FmStatusBadge from './FmStatusBadge.vue'

const props = withDefaults(defineProps<{ facet: Facet; collapsed?: boolean; serenity?: boolean }>(), { collapsed: false, serenity: false })
const open = ref(!props.collapsed)
const rows = computed(() => Object.entries(props.facet.fields ?? {}).sort(([a], [b]) => a.localeCompare(b)))
const fmt = (v: unknown) => (typeof v === 'object' && v !== null ? JSON.stringify(v) : String(v))
</script>

<template>
  <section class="fm-panel" :data-source="facet.source">
    <header class="fm-panel__head">
      <button class="fm-panel__toggle" type="button" :aria-expanded="open" @click="open = !open">
        <span class="fm-panel__chev" :class="{ open }" aria-hidden="true">▸</span>
        <span class="fm-panel__source">{{ facet.source }}</span>
        <span class="fm-panel__title">{{ facet.title }}</span>
      </button>
      <FmStatusBadge :phase="facet.status" size="sm" :serenity="serenity" />
      <div class="fm-panel__links"><FmDeepLink v-for="l in facet.links ?? []" :key="l.url" :link="l" /></div>
    </header>
    <div v-show="open" class="fm-panel__body">
      <slot :facet="facet">
        <table v-if="rows.length" class="fm-kv">
          <tbody><tr v-for="[k, v] in rows" :key="k"><th>{{ k }}</th><td>{{ fmt(v) }}</td></tr></tbody>
        </table>
        <p v-else class="fm-panel__none">This adapter reported no additional fields.</p>
      </slot>
    </div>
  </section>
</template>

<style>
.fm-panel { border: 1px solid var(--fm-border); border-radius: var(--fm-radius); background: var(--fm-bg-elev); font-family: var(--fm-font-sans); min-width: 0; }
.fm-panel__head { display: flex; align-items: center; gap: .6rem; padding: .6rem .8rem; flex-wrap: wrap; }
.fm-panel__toggle { display: inline-flex; align-items: center; gap: .5rem; background: none; border: 0; color: var(--fm-fg); font: inherit; cursor: pointer; padding: 0; min-width: 0; }
.fm-panel__chev { display: inline-block; transition: transform .15s; color: var(--fm-fg-faint); }
.fm-panel__chev.open { transform: rotate(90deg); }
.fm-panel__source { font: 600 .7rem var(--fm-font-mono); color: var(--fm-accent); background: var(--fm-accent-soft); padding: .1rem .45rem; border-radius: 999px; }
.fm-panel__title { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fm-panel__links { margin-left: auto; display: flex; gap: .35rem; }
.fm-panel__body { border-top: 1px solid var(--fm-border); padding: .6rem .8rem; overflow-x: auto; }
.fm-kv { border-collapse: collapse; width: 100%; font-size: .85rem; }
.fm-kv th { text-align: left; font-weight: 500; color: var(--fm-fg-muted); padding: .25rem .6rem .25rem 0; white-space: nowrap; vertical-align: top; font-family: var(--fm-font-mono); font-size: .78rem; }
.fm-kv td { padding: .25rem 0; word-break: break-word; }
.fm-panel__none { margin: 0; color: var(--fm-fg-faint); font-size: .85rem; }
</style>
