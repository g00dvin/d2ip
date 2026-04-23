<script setup lang="ts">
import { toasts } from '@/stores/toast'
</script>

<template>
  <div class="fixed bottom-5 right-5 z-[1000] flex flex-col gap-2">
    <div
      v-for="toast in toasts"
      :key="toast.id"
      :class="[
        'bg-surface-sidebar border px-4 py-2 font-mono text-xs',
        toast.type === 'success' ? 'border-brand text-brand' : 'border-err text-err',
      ]"
    >
      {{ toast.message }}
    </div>
  </div>
</template>