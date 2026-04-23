<script setup lang="ts">
import { computed } from 'vue'
import VueApexCharts from 'vue3-apexcharts'
import { useAppStore } from '@/stores/app'

const props = defineProps<{
  stats: { records_valid: number; records_failed: number } | null
}>()

const app = useAppStore()

const options = computed(() => ({
  theme: { mode: app.isDark ? 'dark' : 'light' as 'dark' | 'light' },
  chart: { type: 'donut' as const, background: 'transparent' },
  labels: ['Valid', 'Failed'],
  legend: { position: 'bottom' as const },
}))

const series = computed(() => {
  if (!props.stats) return [0, 0]
  return [props.stats.records_valid, props.stats.records_failed]
})
</script>

<template>
  <VueApexCharts type="donut" height="250" :options="options" :series="series" />
</template>
