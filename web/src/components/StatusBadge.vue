<script setup lang="ts">
defineProps<{ type: 'ok' | 'warn' | 'error' | 'muted' }>()
</script>

<template>
  <span :class="[
    type === 'ok' ? 'text-ok' : type === 'warn' ? 'text-warn' : type === 'error' ? 'text-err' : 'text-txt-muted',
  ]">
    <slot />
  </span>
</template>