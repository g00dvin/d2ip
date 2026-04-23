<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useToast } from '@/stores/toast'
import { configured, allAvailable, domains, domainsCode, domainsTotal, fetchCategories, addCategory, removeCategory, fetchDomains } from '@/stores/categories'
import StatusBadge from '@/components/StatusBadge.vue'
import { useConfirm } from '@/composables/useConfirm'

const toast = useToast()
const confirm = useConfirm()

const searchTerm = ref('')
const domainFilter = ref('')

onMounted(fetchCategories)

const filteredAvailable = computed(() => {
  const q = searchTerm.value.toLowerCase()
  if (!q) return allAvailable.value
  return allAvailable.value.filter((c) => c.toLowerCase().includes(q))
})

const filteredDomains = computed(() => {
  const q = domainFilter.value.toLowerCase()
  if (!q) return domains.value.slice(0, 100)
  return domains.value.filter((d) => d.toLowerCase().includes(q)).slice(0, 100)
})

async function handleAdd(code: string) {
  try {
    await addCategory(code)
    toast.success('added: ' + code)
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleRemove(code: string) {
  if (!(await confirm.confirm('remove ' + code + '?'))) return
  try {
    await removeCategory(code)
    toast.success('removed: ' + code)
  } catch (e) {
    toast.error((e as Error).message)
  }
}

async function handleBrowse(code: string) {
  try {
    await fetchDomains(code)
  } catch (e) {
    toast.error((e as Error).message)
  }
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">configured categories</div>
      <template v-if="configured.length === 0">
        <StatusBadge type="muted">none</StatusBadge>
        <div class="warning-banner mt-3">
          No categories configured. Search below and click 'add' to start.
        </div>
      </template>
      <template v-else>
        <table class="table-auto">
          <thead>
            <tr><th>code</th><th>domains</th><th>actions</th></tr>
          </thead>
          <tbody>
            <tr v-for="c in configured" :key="c.code">
              <td>{{ c.code }}</td>
              <td>{{ c.domain_count ?? '?' }}</td>
              <td>
                <button class="btn btn-accent text-2xs" @click="handleBrowse(c.code)">browse</button>
                <button class="btn btn-danger text-2xs ml-1" @click="handleRemove(c.code)">✕</button>
              </td>
            </tr>
          </tbody>
        </table>
      </template>
    </div>

    <div class="panel">
      <div class="panel-label">available categories</div>
      <input
        v-model="searchTerm"
        type="text"
        class="form-input"
        placeholder="filter categories..."
      />
      <div class="mt-3">
        <template v-if="filteredAvailable.length === 0">
          <StatusBadge type="muted">all categories configured</StatusBadge>
        </template>
        <template v-else>
          <div
            v-for="cat in filteredAvailable.slice(0, 50)"
            :key="cat"
            class="flex justify-between items-center py-1 border-b border-border"
          >
            <span>{{ cat }}</span>
            <button class="btn btn-accent text-2xs" @click="handleAdd(cat)">add</button>
          </div>
          <div v-if="filteredAvailable.length > 50" class="meta-text">
            ... and {{ filteredAvailable.length - 50 }} more (use search to filter)
          </div>
        </template>
      </div>
    </div>

    <div v-if="domainsCode" class="panel">
      <div class="panel-label">domains: {{ domainsCode }}</div>
      <input
        v-model="domainFilter"
        type="text"
        class="form-input mb-3"
        placeholder="filter domains..."
      />
      <div v-for="d in filteredDomains" :key="d" class="text-xs">{{ d }}</div>
      <div v-if="domains.length > 100 || (domainFilter === '' && domainsTotal > 100)" class="meta-text">
        ... and {{ domainsTotal - 100 }} more (use filter)
      </div>
    </div>
  </div>
</template>