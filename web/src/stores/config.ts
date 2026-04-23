import { ref } from 'vue'
import api, { type SettingsResponse } from '@/api'

export const config = ref<Record<string, unknown>>({})
export const defaults = ref<Record<string, unknown>>({})
export const overrides = ref<Record<string, string>>({})
export const loading = ref(false)

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

export async function fetchSettings() {
  loading.value = true
  try {
    const { data } = await api.get<SettingsResponse>('/api/settings')
    config.value = data.config
    defaults.value = data.defaults
    overrides.value = data.overrides
  } finally {
    loading.value = false
  }
}

export async function saveSettings(newOverrides: Record<string, string>) {
  await api.put('/api/settings', newOverrides)
  await fetchSettings()
}

export async function deleteOverride(key: string) {
  await api.delete(`/api/settings/${encodeURIComponent(key)}`)
  await fetchSettings()
}