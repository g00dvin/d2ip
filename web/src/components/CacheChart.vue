<script setup lang="ts">
import { computed } from 'vue'
import { Doughnut } from 'vue-chartjs'
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js'
ChartJS.register(ArcElement, Tooltip, Legend)

const props = defineProps<{
  stats: { records_valid: number; records_failed: number } | null
}>()

const chartData = computed(() => ({
  labels: ['Valid', 'Failed'],
  datasets: [
    {
      data: props.stats ? [props.stats.records_valid, props.stats.records_failed] : [0, 0],
      backgroundColor: ['#18a058', '#d03050'],
    },
  ],
}))

const options = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { position: 'bottom' as const },
  },
}))
</script>

<template>
  <div style="height: 250px">
    <Doughnut :data="chartData" :options="options" />
  </div>
</template>
