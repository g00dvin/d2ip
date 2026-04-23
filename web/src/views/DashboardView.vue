<script setup lang="ts">
import { onMounted } from 'vue'
import { usePolling } from '@/composables/usePolling'
import { useToast } from '@/stores/toast'
import { fetchHealth, healthStatus } from '@/stores/health'
import { status as pipelineStatus, fetchStatus as fetchPipelineStatus, runPipeline, formatDuration } from '@/stores/pipeline'
import { snapshot, fetchSnapshot } from '@/stores/routing'
import { configured } from '@/stores/categories'
import { fetchCategories } from '@/stores/categories'
import StatusBadge from '@/components/StatusBadge.vue'


const toast = useToast()

const healthPoll = usePolling(fetchHealth, 10_000)
const pipelinePoll = usePolling(fetchPipelineStatus, 10_000)
const routingPoll = usePolling(fetchSnapshot, 30_000)

onMounted(async () => {
  await fetchCategories()
  healthPoll.start()
  pipelinePoll.start()
  routingPoll.start()
})

async function handleRunPipeline() {
  try {
    await runPipeline()
    toast.success('pipeline started')
    await fetchPipelineStatus()
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleForceResolve() {
  try {
    await runPipeline(true)
    toast.success('force resolve started')
    await fetchPipelineStatus()
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
      <StatusBadge v-if="healthStatus === 'healthy'" type="ok">● healthy</StatusBadge>
      <StatusBadge v-else-if="healthStatus === 'unhealthy'" type="error">● unhealthy</StatusBadge>
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
      <template v-if="pipelineStatus?.running">
        <StatusBadge type="warn">● running (id: {{ pipelineStatus.run_id }})</StatusBadge>
      </template>
      <template v-else-if="pipelineStatus?.report">
        <template v-if="pipelineStatus.report.domains === 0">
          <StatusBadge type="warn">⚠ no categories configured</StatusBadge>
        </template>
        <template v-else-if="pipelineStatus.report.failed > 0 && pipelineStatus.report.resolved === 0">
          <StatusBadge type="error">⚠ all resolutions failed</StatusBadge>
        </template>
        <template v-else>
          <StatusBadge type="ok">● completed</StatusBadge>
        </template>
        <div class="meta-text">
          id:{{ pipelineStatus.report.run_id }} |
          {{ formatDuration(pipelineStatus.report.duration) }} |
          {{ pipelineStatus.report.domains }} domains |
          {{ pipelineStatus.report.resolved }} resolved |
          {{ pipelineStatus.report.failed }} failed |
          v4:{{ pipelineStatus.report.ipv4_out }} v6:{{ pipelineStatus.report.ipv6_out }}
        </div>
      </template>
      <template v-else>
        <StatusBadge type="muted">no runs yet</StatusBadge>
      </template>
    </div>

    <div v-if="configured.length === 0" class="panel">
      <div class="warning-banner">
        ⚠ No categories configured.
        <router-link :to="{ name: 'categories' }">Go to Categories →</router-link>
      </div>
    </div>

    <div class="panel">
      <div class="panel-label">routing state</div>
      <template v-if="snapshot">
        <div class="meta-text">backend: {{ snapshot.backend || 'none' }}</div>
        <div class="meta-text">ipv4: {{ snapshot.v4?.length ?? 0 }} prefixes | ipv6: {{ snapshot.v6?.length ?? 0 }} prefixes</div>
        <div class="meta-text">applied: {{ fmtDate(snapshot.applied_at) }}</div>
      </template>
      <template v-else>
        <StatusBadge type="muted">routing disabled</StatusBadge>
      </template>
    </div>
  </div>
</template>