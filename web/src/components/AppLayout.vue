<script setup lang="ts">
import { computed, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { darkTheme } from 'naive-ui'
import { useAppStore } from '@/stores/app'
import { useHealthStore } from '@/stores/health'
import { usePolling } from '@/composables/usePolling'
import {
  AnalyticsOutline, ListOutline, OptionsOutline, CopyOutline,
  ServerOutline, GlobeOutline, NavigateOutline, SunnyOutline, MoonOutline, MenuOutline,
} from '@vicons/ionicons5'

const app = useAppStore()
const health = useHealthStore()
const route = useRoute()
const router = useRouter()

usePolling(() => health.fetchHealth(), 10_000)

const menuItems = [
  { name: 'dashboard', label: 'Dashboard', icon: AnalyticsOutline },
  { name: 'pipeline', label: 'Pipeline', icon: ListOutline },
  { name: 'categories', label: 'Categories', icon: CopyOutline },
  { name: 'config', label: 'Config', icon: OptionsOutline },
  { name: 'cache', label: 'Cache', icon: ServerOutline },
  { name: 'source', label: 'Source', icon: GlobeOutline },
  { name: 'routing', label: 'Routing', icon: NavigateOutline },
]

const activeKey = computed(() => route.name as string)

function handleMenuSelect(key: string) {
  router.push({ name: key })
  app.closeMobileDrawer()
}
</script>

<template>
  <n-config-provider :theme="app.isDark ? darkTheme : null">
    <n-layout has-sider style="height: 100vh">
      <!-- Desktop sidebar -->
      <n-layout-sider
        v-show="!app.mobileDrawerOpen"
        bordered
        collapse-mode="width"
        :collapsed-width="0"
        :width="220"
        :collapsed="false"
        class="hidden md:flex"
      >
        <div class="p-4 font-bold text-lg flex items-center gap-2">
          <span class="text-primary">d2ip</span>
        </div>
        <n-menu
          :value="activeKey"
          :options="menuItems.map(i => ({
            key: i.name,
            label: i.label,
            icon: () => h(i.icon),
          }))"
          @update:value="handleMenuSelect"
        />
      </n-layout-sider>

      <!-- Mobile drawer -->
      <n-drawer v-model:show="app.mobileDrawerOpen" width="220" placement="left" class="md:hidden">
        <n-drawer-content title="d2ip" :native-scrollbar="false">
          <n-menu
            :value="activeKey"
            :options="menuItems.map(i => ({
              key: i.name,
              label: i.label,
              icon: () => h(i.icon),
            }))"
            @update:value="handleMenuSelect"
          />
        </n-drawer-content>
      </n-drawer>

      <n-layout>
        <n-layout-header bordered class="flex items-center justify-between px-4" style="height: 56px">
          <div class="flex items-center gap-3">
            <n-button class="md:hidden" text @click="app.openMobileDrawer">
              <n-icon size="24"><MenuOutline /></n-icon>
            </n-button>
            <span class="font-semibold">{{ menuItems.find(i => i.name === activeKey)?.label ?? 'd2ip' }}</span>
          </div>
          <div class="flex items-center gap-3">
            <n-tag v-if="health.status === 'healthy'" type="success" size="small">● healthy</n-tag>
            <n-tag v-else-if="health.status === 'unhealthy'" type="error" size="small">● unhealthy</n-tag>
            <n-tag v-else type="default" size="small">checking...</n-tag>
            <n-button text @click="app.toggleDark">
              <n-icon size="20">
                <SunnyOutline v-if="app.isDark" />
                <MoonOutline v-else />
              </n-icon>
            </n-button>
          </div>
        </n-layout-header>

        <n-layout-content class="p-4" :native-scrollbar="true">
          <slot />
        </n-layout-content>
      </n-layout>
    </n-layout>
  </n-config-provider>
</template>
