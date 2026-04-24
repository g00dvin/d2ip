<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  stats: { domains_valid: number; domains_failed: number; domains_nxdomain: number } | null
}>()

const total = computed(() =>
  (props.stats?.domains_valid ?? 0) +
  (props.stats?.domains_failed ?? 0) +
  (props.stats?.domains_nxdomain ?? 0)
)
const validPct = computed(() => total.value > 0 ? Math.round((props.stats!.domains_valid / total.value) * 100) : 0)
const failedPct = computed(() => total.value > 0 ? Math.round((props.stats!.domains_failed / total.value) * 100) : 0)
const nxdomainPct = computed(() => total.value > 0 ? Math.round((props.stats!.domains_nxdomain / total.value) * 100) : 0)
</script>

<template>
  <div v-if="stats" class="space-y-3">
    <div>
      <div class="flex justify-between text-sm mb-1">
        <span>Valid</span>
        <span>{{ stats.domains_valid }} ({{ validPct }}%)</span>
      </div>
      <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-4 overflow-hidden">
        <div class="bg-green-500 h-4 rounded transition-all" :style="{ width: validPct + '%' }" />
      </div>
    </div>
    <div>
      <div class="flex justify-between text-sm mb-1">
        <span>Failed</span>
        <span>{{ stats.domains_failed }} ({{ failedPct }}%)</span>
      </div>
      <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-4 overflow-hidden">
        <div class="bg-red-500 h-4 rounded transition-all" :style="{ width: failedPct + '%' }" />
      </div>
    </div>
    <div>
      <div class="flex justify-between text-sm mb-1">
        <span>NXDomain</span>
        <span>{{ stats.domains_nxdomain }} ({{ nxdomainPct }}%)</span>
      </div>
      <div class="w-full bg-gray-200 dark:bg-gray-700 rounded h-4 overflow-hidden">
        <div class="bg-yellow-500 h-4 rounded transition-all" :style="{ width: nxdomainPct + '%' }" />
      </div>
    </div>
  </div>
</template>
