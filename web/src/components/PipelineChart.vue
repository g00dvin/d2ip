<script setup lang="ts">
import { computed } from 'vue'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, Title, Tooltip, Legend, LineElement, PointElement, CategoryScale, LinearScale } from 'chart.js'

ChartJS.register(Title, Tooltip, Legend, LineElement, PointElement, CategoryScale, LinearScale)

const props = defineProps<{
  history: Array<{ run_id: number; resolved: number; duration: number; domains: number }>
}>()

const chartData = computed(() => ({
  labels: props.history.map((_, i) => `Run ${props.history.length - i}`),
  datasets: [
    {
      label: 'Resolved',
      data: props.history.map(h => h.resolved),
      borderColor: '#18a058',
      backgroundColor: '#18a058',
      tension: 0.3,
      yAxisID: 'y',
    },
    {
      label: 'Duration (s)',
      data: props.history.map(h => +(h.duration / 1e9).toFixed(1)),
      borderColor: '#2080f0',
      backgroundColor: '#2080f0',
      tension: 0.3,
      yAxisID: 'y1',
    },
  ],
}))

const options = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { position: 'top' as const },
  },
  scales: {
    y: { type: 'linear' as const, display: true, position: 'left' as const },
    y1: { type: 'linear' as const, display: true, position: 'right' as const, grid: { drawOnChartArea: false } },
  },
}
</script>

<template>
  <div style="height: 250px">
    <Line :data="chartData" :options="options" />
  </div>
</template>
