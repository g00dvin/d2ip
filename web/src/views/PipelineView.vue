<script setup lang="ts">
import { onMounted } from 'vue'
import { useToast } from '@/stores/toast'
import {
  status, history, isRunning,
  fetchStatus, fetchHistory, runPipeline, cancelPipeline, formatDuration,
} from '@/stores/pipeline'
import StatusBadge from '@/components/StatusBadge.vue'
import { useConfirm } from '@/composables/useConfirm'
import { usePolling } from '@/composables/usePolling'

const toast = useToast()
const confirm = useConfirm()

usePolling(fetchStatus, () => isRunning.value ? 2000 : 10000)

onMounted(fetchHistory)

async function handleRun() {
  try {
    await runPipeline()
    toast.success('pipeline started')
    await fetchStatus()
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleForceResolve() {
  try {
    await runPipeline(true)
    toast.success('force resolve started')
    await fetchStatus()
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleCancel() {
  if (!(await confirm.confirm('cancel running pipeline?'))) return
  try {
    await cancelPipeline()
    toast.success('pipeline cancelled')
    await fetchStatus()
  } catch (e) {
    toast.error((e as Error).message)
  }
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">pipeline control</div>
      <div class="flex gap-2 flex-wrap">
        <button class="btn btn-accent" @click="handleRun">▶ run pipeline</button>
        <button class="btn btn-warn" @click="handleForceResolve">⚡ force resolve</button>
        <button class="btn btn-danger" @click="handleCancel">■ cancel</button>
      </div>
    </div>

    <div class="panel">
      <div class="panel-label">current status</div>
      <template v-if="status?.running">
        <StatusBadge type="warn">● running (id: {{ status.run_id }})</StatusBadge>
        <div class="meta-text">started: {{ new Date(status.started).toLocaleString() }}</div>
      </template>
      <template v-else-if="status?.report">
        <template v-if="status.report.domains === 0">
          <StatusBadge type="warn">⚠ nothing to resolve — check categories</StatusBadge>
        </template>
        <template v-else-if="status.report.failed > 0 && status.report.resolved === 0">
          <StatusBadge type="error">⚠ all resolutions failed — check DNS upstream</StatusBadge>
        </template>
        <template v-else>
          <StatusBadge type="ok">● completed</StatusBadge>
        </template>
        <div class="meta-text">
          id:{{ status.report.run_id }} |
          {{ formatDuration(status.report.duration) }} |
          {{ status.report.domains }} domains |
          {{ status.report.resolved }} resolved |
          {{ status.report.failed }} failed |
          v4:{{ status.report.ipv4_out }} v6:{{ status.report.ipv6_out }}
        </div>
      </template>
      <template v-else>
        <StatusBadge type="muted">no runs yet</StatusBadge>
      </template>
    </div>

    <div class="panel">
      <div class="panel-label">run history</div>
      <template v-if="history.length === 0">
        <StatusBadge type="muted">no runs yet</StatusBadge>
      </template>
      <template v-else>
        <table class="table-auto">
          <thead>
            <tr>
              <th>id</th><th>domains</th><th>resolved</th><th>failed</th><th>v4</th><th>v6</th><th>duration</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in [...history].reverse()" :key="r.run_id" :class="r.domains === 0 ? 'text-warn' : ''">
              <td>{{ r.run_id }}</td>
              <td>{{ r.domains }}</td>
              <td>{{ r.resolved }}</td>
              <td>{{ r.failed }}</td>
              <td>{{ r.ipv4_out }}</td>
              <td>{{ r.ipv6_out }}</td>
              <td>{{ formatDuration(r.duration) }}</td>
            </tr>
          </tbody>
        </table>
      </template>
    </div>
  </div>
</template>