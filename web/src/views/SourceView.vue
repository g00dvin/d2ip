<script setup lang="ts">
import { onMounted } from 'vue'
import { useSourceStore } from '@/stores/source'
import { useMessage } from 'naive-ui'

const source = useSourceStore()
const message = useMessage()

onMounted(() => source.fetchInfo())

const fmtDate = (d: string | undefined) => d ? new Date(d).toLocaleString() : 'never'

async function handleFetch() {
  try {
    await source.fetchSource()
    message.success('Source fetched successfully')
  } catch (e) {
    message.error('Failed to fetch source')
  }
}
</script>

<template>
  <div class="space-y-4">
    <n-card title="Source Info">
      <n-spin v-if="source.loading" />
      <n-empty v-else-if="!source.info || !source.info.available" description="No dlc.dat cached yet. Run pipeline to download." />
      <n-descriptions v-else label-placement="top" bordered :columns="2">
        <n-descriptions-item label="Available">
          <n-tag type="success">Yes</n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="Fetched">{{ fmtDate(source.info.fetched_at) }}</n-descriptions-item>
        <n-descriptions-item label="Size">{{ source.info.size }}</n-descriptions-item>
        <n-descriptions-item label="ETag">{{ source.info.etag }}</n-descriptions-item>
        <n-descriptions-item label="SHA256">{{ source.info.sha256 }}</n-descriptions-item>
        <n-descriptions-item label="Last Modified">{{ fmtDate(source.info.last_modified) }}</n-descriptions-item>
      </n-descriptions>
    </n-card>

    <n-card title="Actions">
      <n-button type="primary" :loading="source.fetching" @click="handleFetch">
        Fetch Source
      </n-button>
    </n-card>
  </div>
</template>
