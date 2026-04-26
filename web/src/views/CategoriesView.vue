<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useCategoriesStore } from '@/stores/categories'
import CategoryBrowser from '@/components/CategoryBrowser.vue'

const categories = useCategoriesStore()

const searchTerm = ref('')

onMounted(() => categories.fetchCategories())

const filteredAvailable = computed(() => {
  const q = searchTerm.value.toLowerCase()
  if (!q) return categories.available
  return categories.available.filter(c => c.toLowerCase().includes(q))
})

const groupedCategories = computed(() => {
  const groups: Record<string, string[]> = {}
  for (const cat of filteredAvailable.value) {
    const prefix = cat.split(':')[0] || 'unknown'
    if (!groups[prefix]) groups[prefix] = []
    groups[prefix].push(cat)
  }
  for (const prefix in groups) {
    groups[prefix].sort()
  }
  return groups
})

function browseCategory(code: string) {
  categories.browseCategory(code)
}
</script>

<template>
  <div class="space-y-4">
    <n-card title="Browse Categories">
      <n-input v-model:value="searchTerm" placeholder="Filter categories..." clearable class="mb-3" />
      <n-collapse>
        <n-collapse-item
          v-for="(cats, prefix) in groupedCategories"
          :key="prefix"
          :title="`${prefix} (${cats.length})`"
        >
          <n-list hoverable clickable>
            <n-list-item
              v-for="cat in cats"
              :key="cat"
              @click="browseCategory(cat)"
            >
              {{ cat }}
            </n-list-item>
          </n-list>
        </n-collapse-item>
      </n-collapse>
      <n-text v-if="filteredAvailable.length === 0" type="info">
        No categories match your filter
      </n-text>
    </n-card>

    <!-- Browser Drawer -->
    <CategoryBrowser :data="categories.browserData" @close="categories.closeBrowser" />
  </div>
</template>
