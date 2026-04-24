<script setup lang="ts">
import { ref } from 'vue'
import { usePoliciesStore } from '@/stores/policies'
import { usePolling } from '@/composables/usePolling'
import { useMessage } from 'naive-ui'
import type { PolicyConfig } from '@/api/types'

const policies = usePoliciesStore()
const message = useMessage()

usePolling(() => policies.fetchPolicies(), 30_000)

const showModal = ref(false)
const isEditing = ref(false)
const saving = ref(false)

const emptyPolicy: PolicyConfig = {
  name: '',
  enabled: true,
  categories: [],
  backend: 'none',
  dry_run: false,
  export_format: 'plain',
}

const form = ref<PolicyConfig>({ ...emptyPolicy })

const backendOptions = [
  { label: 'iproute2', value: 'iproute2' },
  { label: 'nftables', value: 'nftables' },
  { label: 'none', value: 'none' },
]

const exportOptions = [
  { label: 'plain', value: 'plain' },
  { label: 'ipset', value: 'ipset' },
  { label: 'json', value: 'json' },
  { label: 'nft', value: 'nft' },
  { label: 'iptables', value: 'iptables' },
  { label: 'bgp', value: 'bgp' },
  { label: 'yaml', value: 'yaml' },
]

const columns = [
  { title: 'Name', key: 'name' },
  { title: 'Backend', key: 'backend' },
  { title: 'Categories', key: 'categories', render: (row: any) => row.categories?.length ?? 0 },
  { title: 'Enabled', key: 'enabled', render: (row: any) => row.enabled ? 'Yes' : 'No' },
  { title: 'Table/Set', key: 'table_set', render: (row: any) => row.table_id || row.nft_set_v4 || '-' },
  {
    title: 'Actions',
    key: 'actions',
    render: (row: PolicyConfig) => {
      return h('div', { class: 'flex gap-2' }, [
        h('button', {
          class: 'text-blue-500 hover:text-blue-700 text-sm',
          onClick: () => editPolicy(row),
        }, 'Edit'),
        h('button', {
          class: 'text-red-500 hover:text-red-700 text-sm',
          onClick: () => deletePolicy(row.name),
        }, 'Delete'),
      ])
    },
  },
]

// Need to import h for render functions
import { h } from 'vue'

function addPolicy() {
  isEditing.value = false
  form.value = { ...emptyPolicy }
  showModal.value = true
}

function editPolicy(policy: PolicyConfig) {
  isEditing.value = true
  form.value = { ...policy }
  showModal.value = true
}

async function deletePolicy(name: string) {
  if (!confirm(`Delete policy "${name}"?`)) return
  try {
    await policies.deletePolicy(name)
    message.success(`Policy "${name}" deleted`)
  } catch (e) {
    message.error('Failed to delete policy')
  }
}

async function savePolicy() {
  if (!form.value.name) {
    message.error('Policy name is required')
    return
  }
  saving.value = true
  try {
    if (isEditing.value) {
      await policies.updatePolicy(form.value.name, form.value)
      message.success('Policy updated')
    } else {
      await policies.createPolicy(form.value)
      message.success('Policy created')
    }
    showModal.value = false
  } catch (e) {
    message.error('Failed to save policy')
  } finally {
    saving.value = false
  }
}

function categoryInput(v: string) {
  form.value.categories = v.split(',').map(s => s.trim()).filter(Boolean)
}

function categoryValue(cats: string[]): string {
  return cats.join(', ')
}
</script>

<template>
  <div class="space-y-4">
    <n-card title="Routing Policies">
      <template #header-extra>
        <n-button type="primary" @click="addPolicy">Add Policy</n-button>
      </template>
      <n-spin v-if="policies.loading" />
      <n-empty v-else-if="policies.policies.length === 0" description="No policies configured" />
      <n-data-table
        v-else
        :columns="columns"
        :data="policies.policies"
        size="small"
      />
    </n-card>

    <n-modal v-model:show="showModal" :title="isEditing ? 'Edit Policy' : 'Add Policy'" preset="card" style="width: 600px; max-width: 90vw">
      <n-form label-placement="left" label-width="140">
        <n-form-item label="Name" required>
          <n-input v-model:value="form.name" :disabled="isEditing" placeholder="e.g. streaming" />
        </n-form-item>
        <n-form-item label="Enabled">
          <n-switch v-model:value="form.enabled" />
        </n-form-item>
        <n-form-item label="Categories">
          <n-input :value="categoryValue(form.categories)" placeholder="netflix, youtube, ..." @update:value="categoryInput" />
        </n-form-item>
        <n-form-item label="Backend">
          <n-select v-model:value="form.backend" :options="backendOptions" />
        </n-form-item>
        <n-form-item v-if="form.backend === 'iproute2'" label="Table ID">
          <n-input-number v-model:value="form.table_id" placeholder="e.g. 100" />
        </n-form-item>
        <n-form-item v-if="form.backend === 'iproute2'" label="Interface">
          <n-input v-model:value="form.iface" placeholder="e.g. eth1" />
        </n-form-item>
        <n-form-item v-if="form.backend === 'nftables'" label="NFT Table">
          <n-input v-model:value="form.nft_table" placeholder="e.g. inet d2ip" />
        </n-form-item>
        <n-form-item v-if="form.backend === 'nftables'" label="NFT Set v4">
          <n-input v-model:value="form.nft_set_v4" placeholder="e.g. policy_v4" />
        </n-form-item>
        <n-form-item v-if="form.backend === 'nftables'" label="NFT Set v6">
          <n-input v-model:value="form.nft_set_v6" placeholder="e.g. policy_v6" />
        </n-form-item>
        <n-form-item label="Export Format">
          <n-select v-model:value="form.export_format" :options="exportOptions" />
        </n-form-item>
        <n-form-item label="Dry Run">
          <n-switch v-model:value="form.dry_run" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showModal = false">Cancel</n-button>
          <n-button type="primary" :loading="saving" @click="savePolicy">Save</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
