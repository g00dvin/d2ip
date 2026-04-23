<script setup lang="ts">
import { usePipelineStore } from '@/stores/pipeline'

const pipeline = usePipelineStore()
</script>

<template>
  <n-card title="Live Progress">
    <n-empty v-if="!pipeline.liveProgress" description="Waiting for progress..." />
    <pre v-else>{{ JSON.stringify(pipeline.liveProgress, null, 2) }}</pre>
  </n-card>
</template>
