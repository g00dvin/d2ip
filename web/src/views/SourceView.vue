<script setup lang="ts">
import { onMounted } from 'vue'
import { useSourceStore } from '@/stores/source'
const source = useSourceStore()
import StatusBadge from '@/components/StatusBadge.vue'

onMounted(() => source.fetchInfo())

function fmtDate(d: string | null | undefined) {
  return d ? new Date(d).toLocaleString() : 'never'
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">source info</div>
      <template v-if="source.info && source.info.available">
        <div class="meta-text">fetched: {{ fmtDate(source.info.fetched_at) }}</div>
        <div class="meta-text">size: {{ source.info.size ?? '?' }} bytes</div>
        <div class="meta-text">etag: {{ source.info.etag ?? 'none' }}</div>
        <div v-if="source.info.sha256" class="meta-text">sha256: {{ source.info.sha256.substring(0, 16) }}...</div>
      </template>
      <template v-else-if="source.info && !source.info.available">
        <StatusBadge type="muted">source not available</StatusBadge>
      </template>
      <template v-else>
        <StatusBadge type="muted">loading...</StatusBadge>
      </template>
    </div>
  </div>
</template>