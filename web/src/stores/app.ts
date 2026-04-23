import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useDark, useToggle } from '@vueuse/core'

export const useAppStore = defineStore('app', () => {
  const isDark = useDark()
  const toggleDark = useToggle(isDark)
  const sidebarCollapsed = ref(false)
  const mobileDrawerOpen = ref(false)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function openMobileDrawer() {
    mobileDrawerOpen.value = true
  }

  function closeMobileDrawer() {
    mobileDrawerOpen.value = false
  }

  return {
    isDark, toggleDark,
    sidebarCollapsed, toggleSidebar,
    mobileDrawerOpen, openMobileDrawer, closeMobileDrawer,
  }
})
