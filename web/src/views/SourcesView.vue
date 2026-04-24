<script setup lang="ts">
import { onMounted, ref, h } from 'vue'
import { useSourcesStore } from '@/stores/sources'
import { useMessage, NButton } from 'naive-ui'
import type { SourceConfig } from '@/api/types'

const store = useSourcesStore()
const message = useMessage()

const showModal = ref(false)
const editing = ref<SourceConfig | null>(null)

onMounted(() => store.fetchSources())

const providerOptions = [
  { label: 'Plaintext (domains/IPs)', value: 'plaintext' },
  { label: 'V2fly Geosite', value: 'v2flygeosite' },
]

function openAdd() {
  editing.value = {
    id: '',
    provider: 'plaintext',
    prefix: '',
    enabled: true,
    config: { type: 'domains', file: '' },
  }
  showModal.value = true
}

async function handleSave() {
  if (!editing.value) return
  if (!editing.value.id || !editing.value.prefix) {
    message.error('ID and prefix are required')
    return
  }
  try {
    await store.addSource(editing.value)
    message.success('Source added')
    showModal.value = false
  } catch (e) {
    message.error('Failed to add source')
  }
}

async function handleDelete(id: string) {
  try {
    await store.removeSource(id)
    message.success('Source deleted')
  } catch (e) {
    message.error('Failed to delete source')
  }
}

async function handleRefresh(id: string) {
  try {
    await store.reloadSource(id)
    message.success('Source refreshed')
  } catch (e) {
    message.error('Failed to refresh source')
  }
}

const columns = [
  { title: 'ID', key: 'id' },
  { title: 'Provider', key: 'provider' },
  { title: 'Prefix', key: 'prefix' },
  { title: 'Categories', key: 'categories', render: (row: any) => row.categories?.length ?? 0 },
  { title: 'Status', key: 'enabled', render: (row: any) => row.enabled ? 'Enabled' : 'Disabled' },
  { title: 'Actions', key: 'actions', render: (row: any) => h('div', { class: 'flex gap-2' }, [
    h(NButton, { size: 'small', onClick: () => handleRefresh(row.id) }, () => 'Refresh'),
    h(NButton, { size: 'small', type: 'error', onClick: () => handleDelete(row.id) }, () => 'Delete'),
  ]) },
]
</script>

<template>
  <div class="space-y-4">
    <n-card title="Sources">
      <template #header-extra>
        <n-button type="primary" @click="openAdd">Add Source</n-button>
      </template>
      <n-spin v-if="store.loading" />
      <n-empty v-else-if="store.sources.length === 0" description="No sources configured" />
      <n-data-table
        v-else
        :columns="columns"
        :data="store.sources"
      />
    </n-card>

    <n-modal v-model:show="showModal" title="Add Source" preset="card" style="width: 500px">
      <n-form v-if="editing" :model="editing">
        <n-form-item label="ID" required>
          <n-input v-model:value="editing.id" placeholder="my-source" />
        </n-form-item>
        <n-form-item label="Prefix" required>
          <n-input v-model:value="editing.prefix" placeholder="corp" />
        </n-form-item>
        <n-form-item label="Provider" required>
          <n-select v-model:value="editing.provider" :options="providerOptions" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'plaintext'" label="Type">
          <n-select v-model:value="editing.config.type" :options="[
            { label: 'Domains', value: 'domains' },
            { label: 'IPs', value: 'ips' },
          ]" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'plaintext'" label="File Path">
          <n-input v-model:value="editing.config.file" placeholder="/var/lib/d2ip/sources/mylist.txt" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'v2flygeosite'" label="URL">
          <n-input v-model:value="editing.config.url" placeholder="https://github.com/..." />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'v2flygeosite'" label="Cache Path">
          <n-input v-model:value="editing.config.cache_path" placeholder="/var/lib/d2ip/dlc.dat" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-button @click="showModal = false">Cancel</n-button>
        <n-button type="primary" @click="handleSave">Save</n-button>
      </template>
    </n-modal>
  </div>
</template>
