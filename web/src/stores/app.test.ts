import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from './app'

describe('app store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('toggles dark mode', () => {
    const store = useAppStore()
    const initial = store.isDark
    store.toggleDark()
    expect(store.isDark).toBe(!initial)
  })

  it('toggles sidebar collapsed state', () => {
    const store = useAppStore()
    expect(store.sidebarCollapsed).toBe(false)
    store.toggleSidebar()
    expect(store.sidebarCollapsed).toBe(true)
    store.toggleSidebar()
    expect(store.sidebarCollapsed).toBe(false)
  })

  it('opens mobile drawer', () => {
    const store = useAppStore()
    expect(store.mobileDrawerOpen).toBe(false)
    store.openMobileDrawer()
    expect(store.mobileDrawerOpen).toBe(true)
  })

  it('closes mobile drawer', () => {
    const store = useAppStore()
    store.openMobileDrawer()
    expect(store.mobileDrawerOpen).toBe(true)
    store.closeMobileDrawer()
    expect(store.mobileDrawerOpen).toBe(false)
  })
})
