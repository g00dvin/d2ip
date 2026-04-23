<script setup lang="ts">
import { useRoute } from 'vue-router'
import { computed } from 'vue'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; navigate: [] }>()

const route = useRoute()

const navItems = [
  { name: 'dashboard', label: 'Dashboard', icon: '▸' },
  { name: 'pipeline', label: 'Pipeline', icon: '▸' },
  { name: 'config', label: 'Config', icon: '▸' },
  { name: 'categories', label: 'Categories', icon: '▸' },
  { name: 'cache', label: 'Cache', icon: '▸' },
  { name: 'source', label: 'Source', icon: '▸' },
  { name: 'routing', label: 'Routing', icon: '▸' },
]

const currentRoute = computed(() => route.name as string)
</script>

<template>
  <aside
    :class="[
      'bg-surface-sidebar border-r border-border flex flex-col shrink-0 transition-transform duration-200 z-40',
      'fixed md:relative h-full md:translate-x-0',
      open ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
    ]"
    class="w-[180px]"
  >
    <div class="p-4 border-b border-border flex items-center gap-2">
      <span class="text-brand text-lg">⬡</span>
      <span class="text-brand font-bold text-base">d2ip</span>
    </div>
    <nav class="flex-1 py-2 overflow-y-auto">
      <router-link
        v-for="item in navItems"
        :key="item.name"
        :to="{ name: item.name }"
        :class="[
          'flex items-center gap-2 px-4 py-1.5 text-txt-secondary hover:text-txt-primary hover:bg-brand/5 transition-colors duration-150 cursor-pointer no-underline',
          currentRoute === item.name ? 'text-brand border-b border-brand' : '',
        ]"
        @click="emit('navigate')"
      >
        <span class="text-[10px]">{{ item.icon }}</span>
        {{ item.label }}
      </router-link>
    </nav>
    <div class="px-4 py-2 border-t border-border flex justify-between text-txt-muted text-[11px]">
      <span>v0.1.4</span>
      <span>:9099</span>
    </div>
  </aside>
  <div
    v-if="open"
    class="fixed inset-0 bg-black/50 z-30 md:hidden"
    @click="emit('close')"
  />
</template>