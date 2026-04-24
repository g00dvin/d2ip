<script setup lang="ts">
import { usePoliciesStore } from '@/stores/policies'
import { usePolling } from '@/composables/usePolling'

const policies = usePoliciesStore()

usePolling(() => policies.fetchPolicies(), 30_000)
</script>

<template>
  <div class="space-y-4">
    <n-card title="Routing Policies">
      <n-spin v-if="policies.loading" />
      <n-empty v-else-if="policies.policies.length === 0" description="No policies configured" />
      <n-data-table
        v-else
        :columns="[
          { title: 'Name', key: 'name' },
          { title: 'Backend', key: 'backend' },
          { title: 'Categories', key: 'categories', render: (row: any) => row.categories?.length ?? 0 },
          { title: 'Enabled', key: 'enabled' },
          { title: 'Table/Set', key: 'table_set', render: (row: any) => row.table_id || row.nft_set_v4 || '-' },
        ]"
        :data="policies.policies"
      />
    </n-card>
  </div>
</template>
