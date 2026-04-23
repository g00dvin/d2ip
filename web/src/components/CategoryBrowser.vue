<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  data: { code: string; domains: string[]; total: number } | null
}>()

const emit = defineEmits(['close'])

const filter = ref('')

const filtered = computed(() => {
  if (!props.data) return []
  const q = filter.value.toLowerCase()
  if (!q) return props.data.domains.slice(0, 500)
  return props.data.domains.filter(d => d.toLowerCase().includes(q)).slice(0, 500)
})
</script>

<template>
  <n-drawer v-if="data" :show="true" @update:show="emit('close')" width="600" style="max-width: 90vw">
    <n-drawer-content :title="`Domains: ${data.code}`" :native-scrollbar="false">
      <n-space vertical>
        <n-input v-model:value="filter" placeholder="Filter domains..." clearable />
        <n-text type="info">Total: {{ data.total }} | Showing: {{ filtered.length }}</n-text>
        <n-virtual-list :item-size="24" :items="filtered" style="height: 60vh">
          <template #default="{ item }">
            <div class="py-1 text-sm border-b border-gray-200 dark:border-gray-700">{{ item }}</div>
          </template>
        </n-virtual-list>
      </n-space>
    </n-drawer-content>
  </n-drawer>
</template>
