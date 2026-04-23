<script setup lang="ts">
import { useSSE } from '@/composables/useSSE'
import { usePipelineStore } from '@/stores/pipeline'
import { useConfigStore } from '@/stores/config'
import { useRoutingStore } from '@/stores/routing'
import AppLayout from '@/components/AppLayout.vue'

const pipeline = usePipelineStore()
const config = useConfigStore()
const routing = useRoutingStore()

useSSE({
  'pipeline.start': (data) => pipeline.handleSSE('pipeline.start', data),
  'pipeline.progress': (data) => pipeline.handleSSE('pipeline.progress', data),
  'pipeline.complete': (data) => pipeline.handleSSE('pipeline.complete', data),
  'pipeline.failed': (data) => pipeline.handleSSE('pipeline.failed', data),
  'pipeline.cancel': (data) => pipeline.handleSSE('pipeline.cancel', data),
  'config.reload': (data) => config.handleSSE('config.reload', data),
  'routing.apply': (data) => routing.handleSSE('routing.apply', data),
})
</script>

<template>
  <AppLayout>
    <router-view />
  </AppLayout>
</template>
