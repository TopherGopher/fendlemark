<script setup lang="ts">
defineProps<{ heading: string; hint?: string }>()
</script>

<template>
  <div class="fm-empty" role="status">
    <p class="fm-empty__heading">{{ heading }}</p>
    <p v-if="hint" class="fm-empty__hint">{{ hint }}</p>
    <slot />
  </div>
</template>

<style>
.fm-empty { padding: 2.5rem 1.5rem; text-align: center; border: 1px dashed var(--fm-border); border-radius: var(--fm-radius); color: var(--fm-fg-muted); font-family: var(--fm-font-sans); }
.fm-empty__heading { margin: 0; color: var(--fm-fg); font-weight: 600; }
.fm-empty__hint { margin: .35rem 0 0; font-size: .9rem; }
</style>
