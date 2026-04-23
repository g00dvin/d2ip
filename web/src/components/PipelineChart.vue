<script setup lang="ts">
import { computed } from 'vue'
import VueApexCharts from 'vue3-apexcharts'
import { useAppStore } from '@/stores/app'

const props = defineProps<{
  history: Array<{ run_id: number; resolved: number; duration: number; domains: number }>
}>()

const app = useAppStore()

const chartOptions = computed(() => ({
  theme: { mode: app.isDark ? 'dark' : 'light' as 'dark' | 'light' },
  chart: {
    type: 'line' as const,
    toolbar: { show: false },
    background: 'transparent',
  },
  xaxis: {
    categories: props.history.map((_, i) => `Run ${props.history.length - i}`),
  },
  yaxis: [
    { title: { text: 'Resolved' } },
    { opposite: true, title: { text: 'Duration (s)' } },
  ],
  stroke: { curve: 'smooth' as const },
  legend: { position: 'top' as const },
}))

const series = computed(() => [
  {
    name: 'Resolved',
    data: props.history.map(h => h.resolved),
  },
  {
    name: 'Duration (s)',
    data: props.history.map(h => +(h.duration / 1e9).toFixed(1)),
  },
])
</script>

<template>
  <VueApexCharts type="line" height="250" :options="chartOptions" :series="series" />
</template>
