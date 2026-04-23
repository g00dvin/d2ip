<script setup lang="ts">
import { onMounted } from 'vue'
import { info, fetchSourceInfo } from '@/stores/source'
import StatusBadge from '@/components/StatusBadge.vue'

onMounted(fetchSourceInfo)

function fmtDate(d: string | null | undefined) {
  return d ? new Date(d).toLocaleString() : 'never'
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">source info</div>
      <template v-if="info && info.available">
        <div class="meta-text">fetched: {{ fmtDate(info.fetched_at) }}</div>
        <div class="meta-text">size: {{ info.size ?? '?' }} bytes</div>
        <div class="meta-text">etag: {{ info.etag ?? 'none' }}</div>
        <div v-if="info.sha256" class="meta-text">sha256: {{ info.sha256.substring(0, 16) }}...</div>
      </template>
      <template v-else-if="info && !info.available">
        <StatusBadge type="muted">source not available</StatusBadge>
      </template>
      <template v-else>
        <StatusBadge type="muted">loading...</StatusBadge>
      </template>
    </div>
  </div>
</template>