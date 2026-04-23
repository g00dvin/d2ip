import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'
import type { SettingsResponse } from '@/api/types'

export const enumFields: Record<string, string[]> = {
  'resolver.network': ['udp', 'tcp', 'tcp-tls'],
  'aggregation.level': ['off', 'conservative', 'balanced', 'aggressive'],
  'routing.backend': ['none', 'nftables', 'iproute2'],
  'logging.format': ['json', 'console'],
  'logging.level': ['debug', 'info', 'warn', 'error'],
}

export const durationFields = new Set([
  'source.http_timeout',
  'resolver.timeout',
  'resolver.backoff_base',
  'resolver.backoff_max',
  'cache.ttl',
  'cache.failed_ttl',
  'scheduler.dlc_refresh',
  'scheduler.resolve_cycle',
])

export const useConfigStore = defineStore('config', () => {
  const settings = ref<SettingsResponse | null>(null)
  const loading = ref(false)
  const error = ref<Error | null>(null)

  async function fetchSettings() {
    loading.value = true
    try {
      const data = await api.getSettings()
      settings.value = data
      error.value = null
    } catch (e) {
      error.value = e as Error
    } finally {
      loading.value = false
    }
  }

  async function updateOverride(key: string, value: string) {
    try {
      await api.updateSettings({ [key]: value })
      await fetchSettings()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  async function saveSettings(newOverrides: Record<string, string>) {
    try {
      await api.updateSettings(newOverrides)
      await fetchSettings()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  async function deleteOverride(key: string) {
    try {
      await api.deleteSetting(key)
      await fetchSettings()
      error.value = null
    } catch (e) {
      error.value = e as Error
      throw e
    }
  }

  function handleSSE(_event: string, _data: unknown) {
    fetchSettings()
  }

  return { settings, loading, error, fetchSettings, updateOverride, saveSettings, deleteOverride, handleSSE }
})
