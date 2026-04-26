<script setup lang="ts">
import { useCacheStore } from '@/stores/cache'
import { usePolling } from '@/composables/usePolling'
import { useConfirm } from '@/composables/useConfirm'
import CacheChart from '@/components/CacheChart.vue'

const cache = useCacheStore()
const confirm = useConfirm()

usePolling(() => cache.fetchStats(), 30_000)

async function handleVacuum() {
  if (!(await confirm.confirm('Run vacuum? This may take a while.'))) return
  await cache.vacuum()
}
</script>

<template>
  <div class="space-y-4">
    <n-card title="Cache Statistics">
      <n-spin v-if="cache.loading" />
      <n-alert v-else-if="cache.error" type="error" :title="cache.error.message" />
      <n-empty v-else-if="!cache.stats" description="No data" />
      <div v-else class="space-y-6">
        <div>
          <h3 class="text-sm font-semibold text-gray-500 mb-2">Domains</h3>
          <n-grid cols="2 s:3 m:4 l:6" :x-gap="16" :y-gap="16">
            <n-gi><n-statistic label="Total" :value="cache.stats.domains" /></n-gi>
            <n-gi><n-statistic label="Valid" :value="cache.stats.domains_valid" /></n-gi>
            <n-gi><n-statistic label="Failed" :value="cache.stats.domains_failed" /></n-gi>
            <n-gi><n-statistic label="NXDomain" :value="cache.stats.domains_nxdomain" /></n-gi>
          </n-grid>
        </div>
        <n-divider />
        <div>
          <h3 class="text-sm font-semibold text-gray-500 mb-2">Records (IP entries)</h3>
          <n-grid cols="2 s:3 m:4 l:6" :x-gap="16" :y-gap="16">
            <n-gi><n-statistic label="Total" :value="cache.stats.records_total" /></n-gi>
            <n-gi><n-statistic label="IPv4" :value="cache.stats.records_v4" /></n-gi>
            <n-gi><n-statistic label="IPv6" :value="cache.stats.records_v6" /></n-gi>
          </n-grid>
        </div>
      </div>
    </n-card>

    <n-card v-if="cache.stats" title="Distribution">
      <CacheChart :stats="cache.stats" />
    </n-card>

    <n-card title="Actions">
      <n-button type="primary" @click="handleVacuum">Vacuum</n-button>
    </n-card>

    <n-modal v-model:show="confirm.visible" preset="dialog" title="Confirm" :content="confirm.message" positive-text="Yes" negative-text="No" @positive-click="confirm.onOk" @negative-click="confirm.onCancel" />
  </div>
</template>
