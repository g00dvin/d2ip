<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useConfigStore, enumFields } from '@/stores/config'
import { useMessage } from 'naive-ui'
import * as api from '@/api/rest'

const config = useConfigStore()
const message = useMessage()

onMounted(() => config.fetchSettings())

const tabs = [
  { name: 'General', keys: ['listen'] },
  { name: 'Source', keys: ['source.url', 'source.cache_path', 'source.refresh_interval', 'source.http_timeout'] },
  { name: 'Resolver', keys: ['resolver.upstream', 'resolver.network', 'resolver.concurrency', 'resolver.qps', 'resolver.timeout', 'resolver.retries', 'resolver.backoff_base', 'resolver.backoff_max', 'resolver.follow_cname', 'resolver.enable_v4', 'resolver.enable_v6'] },
  { name: 'Cache', keys: ['cache.db_path', 'cache.ttl', 'cache.failed_ttl', 'cache.vacuum_after'] },
  { name: 'Aggregation', keys: ['aggregation.enabled', 'aggregation.level', 'aggregation.v4_max_prefix', 'aggregation.v6_max_prefix'] },
  { name: 'Export', keys: ['export.dir', 'export.ipv4_file', 'export.ipv6_file'] },
  { name: 'Routing', keys: ['routing.enabled', 'routing.state_dir'] },
  { name: 'Scheduler', keys: ['scheduler.dlc_refresh', 'scheduler.resolve_cycle'] },
  { name: 'Logging', keys: ['logging.level', 'logging.format'] },
  { name: 'Metrics', keys: ['metrics.enabled', 'metrics.path'] },
]

const durationKeys = new Set([
  'source.refresh_interval',
  'source.http_timeout',
  'resolver.timeout',
  'resolver.backoff_base',
  'resolver.backoff_max',
  'cache.ttl',
  'cache.failed_ttl',
  'cache.vacuum_after',
  'scheduler.dlc_refresh',
  'scheduler.resolve_cycle',
])

function formatDuration(ns: number | string | undefined): string {
  if (ns === undefined || ns === null || ns === '') return ''
  const n = typeof ns === 'string' ? parseFloat(ns) : ns
  if (isNaN(n) || n === 0) return '0s'
  const seconds = n / 1e9
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  const minutes = seconds / 60
  if (minutes < 60) return `${Math.round(minutes)}m`
  const hours = minutes / 60
  if (hours < 24) return `${Math.round(hours)}h`
  const days = hours / 24
  return `${Math.round(days)}d`
}

function parseDurationInput(v: string): string {
  v = v.trim()
  if (!v) return ''
  // If already a number, assume nanoseconds and pass through
  if (/^\d+$/.test(v)) return v
  // Parse human-readable durations like "4h", "30m", "7d"
  const match = v.match(/^(\d+(?:\.\d+)?)\s*([smhd])$/i)
  if (!match) return v
  const num = parseFloat(match[1])
  const unit = match[2].toLowerCase()
  let seconds = num
  if (unit === 'm') seconds = num * 60
  else if (unit === 'h') seconds = num * 3600
  else if (unit === 'd') seconds = num * 86400
  return String(Math.round(seconds * 1e9))
}

// Local edits: key -> current input value
const edits = ref<Record<string, string>>({})

function getValue(key: string): string {
  if (edits.value[key] !== undefined) {
    return edits.value[key]
  }
  const val = config.settings?.overrides[key] ?? (config.settings?.config as any)?.[key] ?? ''
  return val?.toString() ?? ''
}

function isOverridden(key: string): boolean {
  return config.settings?.overrides?.[key] !== undefined
}

function isBooleanField(key: string): boolean {
  return typeof (config.settings?.config as any)?.[key] === 'boolean'
}

function getBooleanOptions() {
  return [
    { label: 'true', value: 'true' },
    { label: 'false', value: 'false' },
  ]
}

function handleChange(key: string, value: string) {
  edits.value[key] = value
}

function handleReset(key: string) {
  delete edits.value[key]
  config.deleteOverride(key)
}

const hasChanges = computed(() => Object.keys(edits.value).length > 0)

async function handleSave() {
  if (!hasChanges.value) return
  try {
    await config.saveSettings(edits.value)
    edits.value = {}
    message.success('Configuration saved')
  } catch (e) {
    message.error('Failed to save configuration')
  }
}

async function handleExport() {
  try {
    const blob = await api.exportConfig()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'd2ip-config.json'
    a.click()
    URL.revokeObjectURL(url)
    message.success('Config exported')
  } catch (e) {
    message.error('Failed to export config')
  }
}

async function handleImport() {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = '.json'
  input.onchange = async () => {
    const file = input.files?.[0]
    if (!file) return
    try {
      const text = await file.text()
      const data = JSON.parse(text)
      const overrides = data.overrides || data.config || {}
      // Flatten if needed
      const flat: Record<string, string> = {}
      for (const [k, v] of Object.entries(overrides)) {
        flat[k] = String(v)
      }
      await api.importConfig(flat)
      await config.fetchSettings()
      message.success('Config imported')
    } catch (e) {
      message.error('Failed to import config')
    }
  }
  input.click()
}
</script>

<template>
  <n-card title="Configuration">
    <n-spin v-if="config.loading" />
    <n-empty v-else-if="!config.settings" description="Failed to load settings" />
    <div v-else class="space-y-4">
      <n-space>
        <n-button type="primary" :disabled="!hasChanges" @click="handleSave">
          Save Changes
        </n-button>
        <n-button @click="handleExport">Export</n-button>
        <n-button @click="handleImport">Import</n-button>
        <n-text v-if="hasChanges" type="warning">
          Unsaved changes
        </n-text>
      </n-space>
      <n-tabs type="line">
        <n-tab-pane v-for="tab in tabs" :key="tab.name" :name="tab.name" :tab="tab.name">
          <n-form label-placement="left" label-width="180">
            <n-alert v-if="tab.name === 'Routing'" type="info" :show-icon="false" class="mb-4">
              Per-policy routing config (backend, table_id, iface, sets) is managed on the
              <router-link to="/policies" class="text-blue-500">Policies</router-link> page.
            </n-alert>
            <n-form-item v-for="key in tab.keys" :key="key" :label="key">
              <n-input-group>
                <!-- Boolean fields: dropdown -->
                <n-select
                  v-if="isBooleanField(key)"
                  :value="getValue(key)"
                  :options="getBooleanOptions()"
                  style="width: 200px"
                  @update:value="(v: string) => handleChange(key, v)"
                />
                <!-- Enum fields: dropdown -->
                <n-select
                  v-else-if="enumFields[key]"
                  :value="getValue(key)"
                  :options="enumFields[key].map((opt: string) => ({ label: opt, value: opt }))"
                  style="width: 200px"
                  @update:value="(v: string) => handleChange(key, v)"
                />
                <!-- Duration fields: show human-readable, store nanoseconds -->
                <n-input
                  v-else-if="durationKeys.has(key)"
                  :value="getValue(key) ? formatDuration(getValue(key)) : ''"
                  :placeholder="formatDuration((config.settings.defaults as any)[key])"
                  @update:value="(v: string) => handleChange(key, parseDurationInput(v))"
                />
                <!-- Regular text fields -->
                <n-input
                  v-else
                  :value="getValue(key)"
                  :placeholder="(config.settings.defaults as any)[key]?.toString() ?? ''"
                  @update:value="(v: string) => handleChange(key, v)"
                />
                <n-button v-if="isOverridden(key)" type="warning" @click="handleReset(key)">Reset</n-button>
              </n-input-group>
            </n-form-item>
          </n-form>
        </n-tab-pane>
      </n-tabs>
    </div>
  </n-card>
</template>
