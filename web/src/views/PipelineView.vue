<script setup lang="ts">
import { onMounted } from 'vue'
import { usePipelineStore } from '@/stores/pipeline'
import { usePolling } from '@/composables/usePolling'
import { useConfirm } from '@/composables/useConfirm'
import PipelineChart from '@/components/PipelineChart.vue'

const pipeline = usePipelineStore()
const confirm = useConfirm()

usePolling(() => pipeline.fetchStatus(), () => pipeline.isRunning ? 2000 : 10000)
onMounted(() => pipeline.fetchHistory())

async function handleRun() {
  await pipeline.runPipeline()
}

async function handleForceResolve() {
  await pipeline.runPipeline({ forceResolve: true })
}

async function handleCancel() {
  if (!(await confirm.confirm('Cancel running pipeline?'))) return
  await pipeline.cancelPipeline()
}
</script>

<template>
  <div class="space-y-4">
    <!-- Controls -->
    <n-card title="Pipeline Control">
      <n-space>
        <n-button type="primary" @click="handleRun" :loading="pipeline.loading" :disabled="pipeline.isRunning">
          Run
        </n-button>
        <n-button type="warning" @click="handleForceResolve" :loading="pipeline.loading" :disabled="pipeline.isRunning">
          Force Resolve
        </n-button>
        <n-button type="error" @click="handleCancel" :disabled="!pipeline.isRunning">
          Cancel
        </n-button>
      </n-space>
    </n-card>

    <!-- Status -->
    <n-card title="Status">
      <n-empty v-if="!pipeline.status" description="No data" />
      <div v-else-if="pipeline.isRunning" class="flex items-center gap-2">
        <n-spin size="small" />
        <span>Running (id: {{ pipeline.status.run_id }})</span>
      </div>
      <n-result v-else-if="pipeline.status.report?.domains === 0" status="warning" title="No categories configured" />
      <n-result v-else status="success" title="Completed">
        <template #description>
          {{ pipeline.status.report?.resolved }} resolved, {{ pipeline.status.report?.failed }} failed
        </template>
      </n-result>
    </n-card>

    <!-- Chart -->
    <n-card v-if="pipeline.history.length > 0" title="History">
      <PipelineChart :history="pipeline.history" />
    </n-card>

    <!-- History Table -->
    <n-card title="Run History">
      <n-data-table
        :columns="[
          { title: 'ID', key: 'run_id' },
          { title: 'Domains', key: 'domains' },
          { title: 'Resolved', key: 'resolved' },
          { title: 'Failed', key: 'failed' },
          { title: 'v4', key: 'ipv4_out' },
          { title: 'v6', key: 'ipv6_out' },
          { title: 'Duration', key: 'duration', render: (row: any) => pipeline.formatDuration(row.duration) },
        ]"
        :data="[...pipeline.history].reverse()"
        :pagination="{ pageSize: 10 }"
        :row-class-name="(row: any) => row.domains === 0 ? 'text-warning' : ''"
      />
    </n-card>

    <!-- Confirm dialog -->
    <n-modal v-model:show="confirm.visible" preset="dialog" title="Confirm" :content="confirm.message" positive-text="Yes" negative-text="No" @positive-click="confirm.onOk" @negative-click="confirm.onCancel" />
  </div>
</template>
