<script setup lang="ts">
import { computed } from 'vue'
import { phaseToken } from '../lattice'

const props = withDefaults(defineProps<{
  /** Pending, Running, Succeeded, Failed, Unknown. */
  phase: string
  /** Render failures in the same calm colour as everything else. Off by default. See FDR-033. */
  serenity?: boolean
  size?: 'sm' | 'md'
}>(), { serenity: false, size: 'md' })

const colour = computed(() => (props.serenity ? 'var(--fm-phase-serene)' : phaseToken(props.phase)))
const label = computed(() => props.phase || 'Unknown')
</script>

<template>
  <span class="fm-badge" :class="[`fm-badge--${size}`, { 'fm-badge--live': phase === 'Running' }]" :style="{ '--c': colour }" :title="label">
    <span class="fm-badge__dot" aria-hidden="true" />{{ label }}
  </span>
</template>

<style>
.fm-badge { display: inline-flex; align-items: center; gap: .4em; font: 500 .8rem var(--fm-font-sans); color: var(--c); background: color-mix(in srgb, var(--c) 12%, transparent); border: 1px solid color-mix(in srgb, var(--c) 30%, transparent); border-radius: 999px; padding: .15em .6em .15em .45em; white-space: nowrap; }
.fm-badge--sm { font-size: .7rem; }
.fm-badge__dot { width: .5em; height: .5em; border-radius: 50%; background: var(--c); }
.fm-badge--live .fm-badge__dot { animation: fm-pulse 1.6s ease-in-out infinite; }
@keyframes fm-pulse { 50% { opacity: .35; } }
@media (prefers-reduced-motion: reduce) { .fm-badge--live .fm-badge__dot { animation: none; } }
</style>
