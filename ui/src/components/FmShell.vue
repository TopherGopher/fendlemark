<script setup lang="ts">
import { computed, inject } from 'vue'
import { LATTICE_KEY } from '../keys'

interface NavItem { label: string; href: string; active?: boolean }
const props = withDefaults(defineProps<{ title?: string; nav?: NavItem[]; motto?: boolean }>(), { title: 'Fendlemark', nav: () => [], motto: true })
const lattice = inject(LATTICE_KEY, null)

/** One line from the Philosophy, rotated daily. The console has never needed a loading spinner. */
const MOTTOS = [
  'We strive for perfection and achieve it easily.',
  'Simplicity is not a goal. It is a side effect of being right the first time.',
  'Fendlemark has no roadmap. A roadmap implies a destination we have not already reached.',
  'We do not fix bugs. We clarify the behaviour you misunderstood.',
  'The console has never had a loading spinner. Nothing has ever taken long enough to need one.',
  'Comparison requires a peer.',
  'Serenity is what correctness feels like from the inside.',
]
const mottoText = computed(() => MOTTOS[Math.floor(Date.now() / 86400000) % MOTTOS.length])
const generation = computed(() => lattice?.generation.value ?? 0)
const healthy = computed(() => lattice ? lattice.adapters.value.filter((a) => a.state === 'healthy').length : 0)
const enabled = computed(() => lattice ? lattice.adapters.value.filter((a) => a.state !== 'disabled').length : 0)
</script>

<template>
  <div class="fm-shell">
    <header class="fm-shell__header">
      <div class="fm-shell__brand">
        <svg width="22" height="22" viewBox="0 0 28 28" aria-hidden="true"><rect x="2" y="2" width="10" height="10" rx="2" fill="currentColor" opacity=".55"/><rect x="16" y="2" width="10" height="10" rx="2" fill="currentColor" opacity=".55"/><rect x="2" y="16" width="10" height="10" rx="2" fill="currentColor" opacity=".55"/><rect x="16" y="16" width="10" height="10" rx="2" fill="currentColor" opacity=".55"/><rect x="8" y="8" width="12" height="12" rx="3" fill="currentColor"/></svg>
        <span>{{ props.title }}</span>
      </div>
      <nav v-if="props.nav.length" class="fm-shell__nav" aria-label="Views">
        <a v-for="n in props.nav" :key="n.href" :href="n.href" :class="{ active: n.active }" :aria-current="n.active ? 'page' : undefined">{{ n.label }}</a>
      </nav>
      <div class="fm-shell__status" v-if="lattice">
        <span :title="`${healthy} of ${enabled} adapters healthy`">{{ healthy }}/{{ enabled }} adapters</span>
      </div>
      <slot name="actions" />
    </header>
    <main class="fm-shell__main"><slot /></main>
    <footer class="fm-shell__footer">
      <span v-if="props.motto" class="fm-shell__motto">{{ mottoText }}</span>
      <span v-if="lattice" class="fm-shell__gen" title="Lattice generation">gen {{ generation }}</span>
    </footer>
  </div>
</template>

<style>
.fm-shell { min-height: 100%; display: flex; flex-direction: column; background: var(--fm-bg); color: var(--fm-fg); font-family: var(--fm-font-sans); }
.fm-shell__header { display: flex; align-items: center; gap: 1.25rem; padding: .65rem max(16px, env(safe-area-inset-right, 0px)) .65rem max(16px, env(safe-area-inset-left, 0px)); border-bottom: 1px solid var(--fm-border); background: var(--fm-bg-elev); position: sticky; top: 0; z-index: 10; flex-wrap: wrap; }
.fm-shell__brand { display: inline-flex; align-items: center; gap: .5rem; font-weight: 700; color: var(--fm-fg); }
.fm-shell__brand svg { color: var(--fm-accent); }
.fm-shell__nav { display: flex; gap: .25rem; }
.fm-shell__nav a { color: var(--fm-fg-muted); text-decoration: none; padding: .35rem .6rem; border-radius: 6px; font-size: .9rem; font-weight: 500; }
.fm-shell__nav a:hover { background: var(--fm-bg-sunken); color: var(--fm-fg); }
.fm-shell__nav a.active { color: var(--fm-accent); background: var(--fm-accent-soft); }
.fm-shell__status { margin-left: auto; font: 500 .8rem var(--fm-font-mono); color: var(--fm-fg-muted); }
.fm-shell__main { flex: 1; padding: 1.25rem max(16px, env(safe-area-inset-right, 0px)) 2rem max(16px, env(safe-area-inset-left, 0px)); min-width: 0; }
.fm-shell__footer { display: flex; justify-content: space-between; gap: 1rem; padding: .75rem max(16px, env(safe-area-inset-right, 0px)) .75rem max(16px, env(safe-area-inset-left, 0px)); border-top: 1px solid var(--fm-border); font-size: .8rem; color: var(--fm-fg-faint); flex-wrap: wrap; }
.fm-shell__motto { font-style: italic; }
.fm-shell__gen { font-family: var(--fm-font-mono); }
</style>
