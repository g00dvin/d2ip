<script setup lang="ts">
import { onMounted } from 'vue'
import { usePolling } from '@/composables/usePolling'
import { useToast } from '@/stores/toast'
import { useHealthStore } from '@/stores/health'
import { usePipelineStore } from '@/stores/pipeline'
import { useRoutingStore } from '@/stores/routing'
import { useCategoriesStore } from '@/stores/categories'

const health = useHealthStore()
const pipeline = usePipelineStore()
const routing = useRoutingStore()
const categories = useCategoriesStore()
import StatusBadge from '@/components/StatusBadge.vue'


const toast = useToast()

const healthPoll = usePolling(() => health.fetchHealth(), 10_000)
const pipelinePoll = usePolling(() => pipeline.fetchStatus(), 10_000)
const routingPoll = usePolling(() => routing.fetchSnapshot(), 30_000)

onMounted(async () => {
  await categories.fetchCategories()
  healthPoll.start()
  pipelinePoll.start()
  routingPoll.start()
})

async function handleRunPipeline() {
  try {
    await pipeline.runPipeline()
    toast.success('pipeline started')
    await pipeline.fetchStatus()
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleForceResolve() {
  try {
    await pipeline.runPipeline({ forceResolve: true })
    toast.success('force resolve started')
    await pipeline.fetchStatus()
  } catch (e) {
    toast.error((e as Error).message)
  }
}



function fmtDate(d: string | null | undefined) {
  return d ? new Date(d).toLocaleString() : 'never'
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">system status</div>
      <StatusBadge v-if="health.status === 'healthy'" type="ok">● healthy</StatusBadge>
      <StatusBadge v-else-if="health.status === 'unhealthy'" type="error">● unhealthy</StatusBadge>
      <StatusBadge v-else type="muted">checking...</StatusBadge>
    </div>

    <div class="panel">
      <div class="panel-label">quick actions</div>
      <div class="flex gap-2 flex-wrap">
        <button class="btn btn-accent" @click="handleRunPipeline">▶ run pipeline</button>
        <button class="btn btn-warn" @click="handleForceResolve">⚡ force resolve</button>
      </div>
    </div>

    <div class="panel">
      <div class="panel-label">last run</div>
      <template v-if="pipeline.status?.running">
        <StatusBadge type="warn">● running (id: {{ pipeline.status.run_id }})</StatusBadge>
      </template>
      <template v-else-if="pipeline.status?.report">
        <template v-if="pipeline.status.report.domains === 0">
          <StatusBadge type="warn">⚠ no categories configured</StatusBadge>
        </template>
        <template v-else-if="pipeline.status.report.failed > 0 && pipeline.status.report.resolved === 0">
          <StatusBadge type="error">⚠ all resolutions failed</StatusBadge>
        </template>
        <template v-else>
          <StatusBadge type="ok">● completed</StatusBadge>
        </template>
        <div class="meta-text">
          id:{{ pipeline.status.report.run_id }} |
          {{ pipeline.formatDuration(pipeline.status.report.duration) }} |
          {{ pipeline.status.report.domains }} domains |
          {{ pipeline.status.report.resolved }} resolved |
          {{ pipeline.status.report.failed }} failed |
          v4:{{ pipeline.status.report.ipv4_out }} v6:{{ pipeline.status.report.ipv6_out }}
        </div>
      </template>
      <template v-else>
        <StatusBadge type="muted">no runs yet</StatusBadge>
      </template>
    </div>

    <div v-if="categories.configured.length === 0" class="panel">
      <div class="warning-banner">
        ⚠ No categories configured.
        <router-link :to="{ name: 'categories' }">Go to Categories →</router-link>
      </div>
    </div>

    <div class="panel">
      <div class="panel-label">routing state</div>
      <template v-if="routing.snapshot">
        <div class="meta-text">backend: {{ routing.snapshot.backend || 'none' }}</div>
        <div class="meta-text">ipv4: {{ routing.snapshot.v4?.length ?? 0 }} prefixes | ipv6: {{ routing.snapshot.v6?.length ?? 0 }} prefixes</div>
        <div class="meta-text">applied: {{ fmtDate(routing.snapshot.applied_at) }}</div>
      </template>
      <template v-else>
        <StatusBadge type="muted">routing disabled</StatusBadge>
      </template>
    </div>
  </div>
</template>