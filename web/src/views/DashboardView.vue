<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { usePipelineStore } from '@/stores/pipeline'
import { useCategoriesStore } from '@/stores/categories'
import { useRoutingStore } from '@/stores/routing'
import { usePoliciesStore } from '@/stores/policies'
import { usePolling } from '@/composables/usePolling'
import { useMessage } from 'naive-ui'
import * as api from '@/api/rest'
import LiveProgress from '@/components/LiveProgress.vue'

const pipeline = usePipelineStore()
const categories = useCategoriesStore()
const routing = useRoutingStore()
const policies = usePoliciesStore()
const message = useMessage()

usePolling(() => pipeline.fetchStatus(), 10_000)
usePolling(() => routing.fetchSnapshot(), 30_000)
onMounted(() => {
  categories.fetchCategories()
  policies.fetchPolicies()
})

const report = computed(() => pipeline.status?.report)
const fmtDate = (d: string | null | undefined) => d ? new Date(d).toLocaleString() : 'never'

async function handleRun() {
  await pipeline.runPipeline()
}

async function handleForceResolve() {
  await pipeline.runPipeline({ forceResolve: true })
}

async function downloadExport(policy: string, type: 'ipv4' | 'ipv6') {
  try {
    const blob = await api.downloadExport(policy, type)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${policy}_${type}.txt`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    message.error(`Failed to download ${type} for ${policy}`)
  }
}
</script>

<template>
  <div class="space-y-4">
    <!-- Quick Actions -->
    <n-card title="Quick Actions">
      <n-space>
        <n-button type="primary" @click="handleRun" :loading="pipeline.loading">
          Run Pipeline
        </n-button>
        <n-button type="warning" @click="handleForceResolve" :loading="pipeline.loading">
          Force Resolve
        </n-button>
      </n-space>
    </n-card>

    <!-- Live Progress -->
    <LiveProgress v-if="pipeline.isRunning || pipeline.liveProgress" />

    <!-- Last Run -->
    <n-card title="Last Run">
      <n-empty v-if="!report" description="No runs yet" />
      <n-descriptions v-else :columns="3" label-placement="top" bordered>
        <n-descriptions-item label="Status">
          <n-tag v-if="pipeline.isRunning" type="warning">Running</n-tag>
          <n-tag v-else-if="report.domains === 0" type="warning">No categories</n-tag>
          <n-tag v-else-if="report.failed > 0 && report.resolved === 0 && report.cache_hits === 0" type="error">All failed</n-tag>
          <n-tag v-else-if="report.failed > 0" type="warning">Completed with errors</n-tag>
          <n-tag v-else type="success">Completed</n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="Duration">{{ pipeline.formatDuration(report.duration) }}</n-descriptions-item>
        <n-descriptions-item label="Domains">{{ report.domains }}</n-descriptions-item>
        <n-descriptions-item label="Resolved">{{ report.resolved }}</n-descriptions-item>
        <n-descriptions-item label="Cache Hits">{{ report.cache_hits ?? 0 }}</n-descriptions-item>
        <n-descriptions-item label="Failed">{{ report.failed }}</n-descriptions-item>
        <n-descriptions-item label="Output">v4: {{ report.ipv4_out }} | v6: {{ report.ipv6_out }}</n-descriptions-item>
      </n-descriptions>
    </n-card>

    <!-- Export Downloads -->
    <n-card v-if="policies.policies.length > 0" title="Export Files">
      <n-space vertical>
        <div v-for="policy in policies.policies" :key="policy.name" class="flex items-center justify-between py-1">
          <span class="font-medium">{{ policy.name }}</span>
          <n-space>
            <n-button size="small" @click="downloadExport(policy.name, 'ipv4')">IPv4</n-button>
            <n-button size="small" @click="downloadExport(policy.name, 'ipv6')">IPv6</n-button>
          </n-space>
        </div>
      </n-space>
    </n-card>

    <!-- Routing State -->
    <n-card title="Routing State">
      <n-empty v-if="!routing.snapshot" description="Routing disabled" />
      <n-descriptions v-else :columns="2" label-placement="top">
        <n-descriptions-item label="Backend">{{ routing.snapshot.backend }}</n-descriptions-item>
        <n-descriptions-item label="Applied">{{ fmtDate(routing.snapshot.applied_at) }}</n-descriptions-item>
        <n-descriptions-item label="IPv4 Prefixes">
          {{ typeof routing.snapshot.v4 === 'number' ? routing.snapshot.v4 : (routing.snapshot.v4?.length ?? 0) }}
        </n-descriptions-item>
        <n-descriptions-item label="IPv6 Prefixes">
          {{ typeof routing.snapshot.v6 === 'number' ? routing.snapshot.v6 : (routing.snapshot.v6?.length ?? 0) }}
        </n-descriptions-item>
        <n-descriptions-item v-if="routing.snapshot.policies !== undefined" label="Policies">
          {{ routing.snapshot.policies }}
        </n-descriptions-item>
      </n-descriptions>
    </n-card>
  </div>
</template>
