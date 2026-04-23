<script setup lang="ts">
import { ref } from 'vue'
import { useRoutingStore } from '@/stores/routing'
import { usePolling } from '@/composables/usePolling'
import { useConfirm } from '@/composables/useConfirm'

const routing = useRoutingStore()
const confirm = useConfirm()

const ipv4Input = ref('')
const ipv6Input = ref('')

usePolling(() => routing.fetchSnapshot(), 30_000)

async function handleDryRun() {
  const ipv4 = ipv4Input.value.split('\n').map(s => s.trim()).filter(Boolean)
  const ipv6 = ipv6Input.value.split('\n').map(s => s.trim()).filter(Boolean)
  await routing.dryRun(ipv4, ipv6)
}

async function handleRollback() {
  if (!(await confirm.confirm('Rollback routing changes?'))) return
  await routing.rollback()
}
</script>

<template>
  <div class="space-y-4">
    <n-card title="Routing State">
      <n-empty v-if="!routing.snapshot" description="Routing disabled" />
      <n-descriptions v-else label-placement="top" :columns="2">
        <n-descriptions-item label="Backend">{{ routing.snapshot.backend }}</n-descriptions-item>
        <n-descriptions-item label="Applied">{{ routing.snapshot.applied_at ? new Date(routing.snapshot.applied_at).toLocaleString() : 'never' }}</n-descriptions-item>
        <n-descriptions-item label="IPv4">{{ routing.snapshot.v4?.length ?? 0 }} prefixes</n-descriptions-item>
        <n-descriptions-item label="IPv6">{{ routing.snapshot.v6?.length ?? 0 }} prefixes</n-descriptions-item>
      </n-descriptions>
    </n-card>

    <n-card title="Dry Run">
      <n-space vertical>
        <n-input v-model:value="ipv4Input" type="textarea" placeholder="IPv4 prefixes (one per line)..." />
        <n-input v-model:value="ipv6Input" type="textarea" placeholder="IPv6 prefixes (one per line)..." />
        <n-button type="primary" @click="handleDryRun" :loading="routing.loading">Preview Changes</n-button>
      </n-space>
    </n-card>

    <n-card v-if="routing.dryRunResult" title="Preview Result">
      <n-descriptions label-placement="top">
        <n-descriptions-item label="v4 Add">{{ routing.dryRunResult.v4_plan.add.length }}</n-descriptions-item>
        <n-descriptions-item label="v4 Remove">{{ routing.dryRunResult.v4_plan.remove.length }}</n-descriptions-item>
        <n-descriptions-item label="v6 Add">{{ routing.dryRunResult.v6_plan.add.length }}</n-descriptions-item>
        <n-descriptions-item label="v6 Remove">{{ routing.dryRunResult.v6_plan.remove.length }}</n-descriptions-item>
      </n-descriptions>
      <n-collapse>
        <n-collapse-item title="v4 diff">
          <pre>{{ routing.dryRunResult.v4_diff }}</pre>
        </n-collapse-item>
        <n-collapse-item title="v6 diff">
          <pre>{{ routing.dryRunResult.v6_diff }}</pre>
        </n-collapse-item>
      </n-collapse>
    </n-card>

    <n-card title="Actions">
      <n-button type="error" @click="handleRollback">Rollback</n-button>
    </n-card>

    <n-modal v-model:show="confirm.visible" preset="dialog" title="Confirm" :content="confirm.message" positive-text="Yes" negative-text="No" @positive-click="confirm.onOk" @negative-click="confirm.onCancel" />
  </div>
</template>
