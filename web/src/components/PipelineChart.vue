<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  history: Array<{ run_id: number; resolved: number; duration: number; domains: number }>
}>()

const maxResolved = computed(() => Math.max(...props.history.map(h => h.resolved), 1))
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="run in [...history].reverse().slice(-20)"
      :key="run.run_id"
      class="flex items-center gap-3 text-sm"
    >
      <span class="w-12 text-gray-500">#{{ run.run_id }}</span>
      <div class="flex-1 bg-gray-200 dark:bg-gray-700 rounded h-5 overflow-hidden relative">
        <div
          class="bg-blue-500 h-5 rounded transition-all"
          :style="{ width: Math.round((run.resolved / maxResolved) * 100) + '%' }"
        />
        <span class="absolute inset-0 flex items-center px-2 text-xs text-white mix-blend-difference">
          {{ run.resolved }} resolved
        </span>
      </div>
      <span class="w-16 text-right text-gray-500">{{ (run.duration / 1e9).toFixed(1) }}s</span>
    </div>
  </div>
</template>
