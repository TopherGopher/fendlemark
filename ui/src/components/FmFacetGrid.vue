<script setup lang="ts">
import { computed } from 'vue'
import type { Entity } from '../types'
import FmFacetPanel from './FmFacetPanel.vue'

const props = withDefaults(defineProps<{ entity: Entity; hideEmpty?: boolean; serenity?: boolean }>(), { hideEmpty: true, serenity: false })
const SOURCE_ORDER = ['argo', 'kubeflow', 'metaflow', 'mlflow']
const facets = computed(() =>
  [...props.entity.facets]
    .filter((f) => !props.hideEmpty || Object.keys(f.fields ?? {}).length || (f.links ?? []).length)
    .sort((a, b) => SOURCE_ORDER.indexOf(a.source) - SOURCE_ORDER.indexOf(b.source) || a.source.localeCompare(b.source)),
)
</script>

<template>
  <div class="fm-grid">
    <FmFacetPanel v-for="f in facets" :key="f.source + f.source_id" :facet="f" :serenity="serenity">
      <template #default="slotProps"><slot :name="f.source" v-bind="slotProps" /></template>
    </FmFacetPanel>
  </div>
</template>

<style>
.fm-grid { display: grid; gap: .75rem; grid-template-columns: 1fr; }
@media (min-width: 900px) { .fm-grid { grid-template-columns: repeat(auto-fit, minmax(26rem, 1fr)); } }
</style>
