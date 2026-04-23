<script setup lang="ts">
import { h } from 'vue'
import { NButton } from 'naive-ui'
import { onMounted, ref, computed } from 'vue'
import { useCategoriesStore } from '@/stores/categories'
import { useConfirm } from '@/composables/useConfirm'
import CategoryBrowser from '@/components/CategoryBrowser.vue'

const categories = useCategoriesStore()
const confirm = useConfirm()

const searchTerm = ref('')

onMounted(() => categories.fetchCategories())

const filteredAvailable = computed(() => {
  const q = searchTerm.value.toLowerCase()
  if (!q) return categories.available
  return categories.available.filter(c => c.toLowerCase().includes(q))
})

async function handleAdd(code: string) {
  await categories.addCategory(code)
}

async function handleRemove(code: string) {
  if (!(await confirm.confirm(`Remove ${code}?`))) return
  await categories.removeCategory(code)
}

async function handleBrowse(code: string) {
  await categories.browseCategory(code)
}
</script>

<template>
  <div class="space-y-4">
    <n-grid cols="1 m:2" :x-gap="16" :y-gap="16">
      <!-- Configured -->
      <n-gi>
        <n-card title="Configured Categories">
          <n-empty v-if="!categories.hasCategories" description="No categories configured" />
          <n-data-table
            v-else
            :columns="[
              { title: 'Code', key: 'code' },
              { title: 'Domains', key: 'domain_count' },
              { title: 'Actions', key: 'actions', render: (row: any) => h('div', [
                h(NButton, { size: 'tiny', type: 'info', onClick: () => handleBrowse(row.code) }, { default: () => 'Browse' }),
                h(NButton, { size: 'tiny', type: 'error', class: 'ml-2', onClick: () => handleRemove(row.code) }, { default: () => 'Remove' }),
              ]) },
            ]"
            :data="categories.configured"
            :pagination="false"
          />
        </n-card>
      </n-gi>

      <!-- Available -->
      <n-gi>
        <n-card title="Available Categories">
          <n-input v-model:value="searchTerm" placeholder="Filter categories..." clearable class="mb-3" />
          <n-list hoverable clickable>
            <n-list-item v-for="cat in filteredAvailable.slice(0, 50)" :key="cat">
              <template #suffix>
                <n-button size="tiny" type="primary" @click="handleAdd(cat)">Add</n-button>
              </template>
              {{ cat }}
            </n-list-item>
          </n-list>
          <n-text v-if="filteredAvailable.length > 50" type="info">
            ... and {{ filteredAvailable.length - 50 }} more
          </n-text>
        </n-card>
      </n-gi>
    </n-grid>

    <!-- Browser Drawer -->
    <CategoryBrowser :data="categories.browserData" @close="categories.closeBrowser" />

    <!-- Confirm -->
    <n-modal v-model:show="confirm.visible" preset="dialog" title="Confirm" :content="confirm.message" positive-text="Yes" negative-text="No" @positive-click="confirm.onOk" @negative-click="confirm.onCancel" />
  </div>
</template>
