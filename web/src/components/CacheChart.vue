<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  stats: { records_valid: number; records_failed: number } | null
}>()

const total = computed(() => (props.stats?.records_valid ?? 0) + (props.stats?.records_failed ?? 0))
const validPct = computed(() => total.value > 0 ? Math.round((props.stats!.records_valid / total.value) * 100) : 0)
const failedPct = computed(() => total.value > 0 ? Math.round((props.stats!.records_failed / total.value) * 100) : 0)
</script>

<template>
  <div v-if="stats" class="space-y-3">
    <div>
      <div class="flex justify-between text-sm mb-1">
        <span>Valid</span>
        <span>{{ stats.records_valid }} ({{ validPct }}%)</span>
      </div>
      <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-4 overflow-hidden">
        <div class="bg-green-500 h-4 rounded transition-all" :style="{ width: validPct + '%' }" />
      </div>
    </div>
    <div>
      <div class="flex justify-between text-sm mb-1">
        <span>Failed</span>
        <span>{{ stats.records_failed }} ({{ failedPct }}%)</span>
      </div>
      <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-4 overflow-hidden">
        <div class="bg-red-500 h-4 rounded transition-all" :style="{ width: failedPct + '%' }" />
      </div>
    </div>
  </div>
</template>
