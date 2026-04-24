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
      <n-empty v-else-if="!cache.stats" description="No data" />
      <n-grid v-else cols="2 s:3 m:4 l:6" :x-gap="16" :y-gap="16">
        <n-gi><n-statistic label="Domains" :value="cache.stats.domains" /></n-gi>
        <n-gi><n-statistic label="Records" :value="cache.stats.records_total" /></n-gi>
        <n-gi><n-statistic label="Valid" :value="cache.stats.records_valid" /></n-gi>
        <n-gi><n-statistic label="Failed" :value="cache.stats.records_failed" /></n-gi>
        <n-gi><n-statistic label="NXDomain" :value="cache.stats.records_nxdomain" /></n-gi>
      </n-grid>
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
