<script setup lang="ts">
import { computed } from 'vue'
import { usePipelineStore } from '@/stores/pipeline'

const pipeline = usePipelineStore()

const progress = computed(() => {
  const lp = pipeline.liveProgress
  if (!lp) return null
  const total = (lp.total as number) || 1
  const resolved = (lp.resolved as number) || 0
  return Math.min(100, Math.round((resolved / total) * 100))
})

const step = computed(() => {
  return (pipeline.liveProgress?.step as string) || 'running'
})
</script>

<template>
  <n-card v-if="pipeline.isRunning || pipeline.liveProgress" title="Pipeline Progress">
    <n-space vertical>
      <n-text>Step: <n-tag type="info">{{ step }}</n-tag></n-text>
      <n-progress
        type="line"
        :percentage="progress ?? 0"
        :indicator-placement="'inside'"
        :processing="pipeline.isRunning"
        :status="pipeline.isRunning ? 'warning' : 'success'"
      />
      <n-space>
        <n-statistic label="Resolved" :value="pipeline.liveProgress?.resolved ?? 0" />
        <n-statistic label="Failed" :value="pipeline.liveProgress?.failed ?? 0" />
        <n-statistic label="Total" :value="pipeline.liveProgress?.total ?? 0" />
      </n-space>
    </n-space>
  </n-card>
</template>
