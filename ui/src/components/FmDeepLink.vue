<script setup lang="ts">
import type { Link } from '../types'
defineProps<{ link: Link }>()
</script>

<template>
  <a class="fm-deeplink" :href="link.url" target="_blank" rel="noopener">
    {{ link.label }}<span aria-hidden="true"> ↗</span>
  </a>
</template>

<style>
.fm-deeplink { font: 500 .82rem var(--fm-font-sans); color: var(--fm-accent); text-decoration: none; border: 1px solid var(--fm-border); border-radius: 6px; padding: .2rem .55rem; background: var(--fm-bg-elev); }
.fm-deeplink:hover { border-color: var(--fm-accent); }
</style>
