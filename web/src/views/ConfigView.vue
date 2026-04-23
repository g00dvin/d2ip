<script setup lang="ts">
import { onMounted, computed, ref, watch } from 'vue'
import { useToast } from '@/stores/toast'
import {
  config, overrides, enumFields, durationFields,
  fetchSettings, saveSettings, deleteOverride,
} from '@/stores/config'

const toast = useToast()

onMounted(fetchSettings)

const sections = computed(() => {
  const order = ['source', 'resolver', 'cache', 'aggregation', 'export', 'routing', 'scheduler', 'logging', 'metrics']
  return order.filter((s) => config.value[s] !== undefined)
})

type FieldDef = { key: string; value: unknown; dottedKey: string; isOverride: boolean }

function getFields(section: string): FieldDef[] {
  const secData = config.value[section] as Record<string, unknown> | undefined
  if (!secData || typeof secData !== 'object') return []
  return Object.entries(secData)
    .filter(([, v]) => typeof v !== 'object' || Array.isArray(v))
    .map(([key, value]) => {
      const dottedKey = `${section}.${key}`
      const isOverride = dottedKey in overrides.value
      const displayValue = isOverride ? overrides.value[dottedKey] : value
      return { key, value: displayValue, dottedKey, isOverride }
    })
}

// Convert nanoseconds to human-readable duration string.
function formatDuration(ns: number): string {
  const seconds = Math.floor(ns / 1e9)
  if (seconds % 3600 === 0 && seconds >= 3600) return `${seconds / 3600}h`
  if (seconds % 60 === 0 && seconds >= 60) return `${seconds / 60}m`
  return `${seconds}s`
}

const formData = ref<Record<string, string>>({})

// Build formData from config whenever settings load.
watch(() => config.value, (cfg) => {
  if (!cfg || !cfg.source) return
  const result: Record<string, string> = {}
  for (const section of sections.value) {
    for (const field of getFields(section)) {
      const dk = field.dottedKey
      if (typeof field.value === 'boolean') {
        result[dk] = field.value ? 'true' : 'false'
      } else if (typeof field.value === 'number' && durationFields.has(dk)) {
        result[dk] = formatDuration(field.value as number)
      } else {
        result[dk] = String(field.value ?? '')
      }
    }
  }
  formData.value = result
}, { immediate: true })

async function handleSave() {
  // Only send fields that differ from current config values.
  const toSend: Record<string, string> = {}
  for (const section of sections.value) {
    for (const field of getFields(section)) {
      const dk = field.dottedKey
      const original = field.isOverride
        ? overrides.value[dk]
        : (typeof field.value === 'number' && durationFields.has(dk))
          ? formatDuration(field.value as number)
          : String(field.value ?? '')
      if (formData.value[dk] !== original) {
        toSend[dk] = formData.value[dk]
      }
    }
  }
  if (Object.keys(toSend).length === 0) {
    toast.success('no changes to save')
    return
  }
  try {
    await saveSettings(toSend)
    toast.success('config saved')
  } catch (e) {
    toast.error('save failed: ' + (e as Error).message)
  }
}

async function handleReset(key: string) {
  try {
    await deleteOverride(key)
    toast.success('override removed: ' + key)
  } catch (e) {
    toast.error((e as Error).message)
  }
}

function isEnum(dottedKey: string): boolean {
  return dottedKey in enumFields
}

function isDuration(dottedKey: string): boolean {
  return durationFields.has(dottedKey)
}
</script>

<template>
  <div>
    <div class="panel">
      <div class="panel-label">configuration</div>
      <template v-if="!config || !config.source">
        <span class="status-muted">loading...</span>
      </template>
      <template v-else>
        <div v-for="section in sections" :key="section" class="panel">
          <div class="panel-label">{{ section }}</div>
          <div v-for="field in getFields(section)" :key="field.dottedKey" class="form-group">
            <label class="form-label" :for="'field-' + field.dottedKey.replace(/\./g, '-')">
              {{ field.key }}{{ field.isOverride ? ' *' : '' }}
            </label>

            <select
              v-if="isEnum(field.dottedKey)"
              :id="'field-' + field.dottedKey.replace(/\./g, '-')"
              :data-key="field.dottedKey"
              v-model="formData[field.dottedKey]"
              class="form-select"
            >
              <option v-for="opt in enumFields[field.dottedKey]" :key="opt" :value="opt">{{ opt }}</option>
            </select>

            <label
              v-else-if="typeof field.value === 'boolean'"
              class="flex items-center gap-2 cursor-pointer"
            >
              <input
                type="checkbox"
                :data-key="field.dottedKey"
                :checked="formData[field.dottedKey] === 'true'"
                @change="formData[field.dottedKey] = ($event.target as HTMLInputElement).checked ? 'true' : 'false'"
                class="accent-brand w-4 h-4 cursor-pointer"
              />
              {{ formData[field.dottedKey] === 'true' ? 'true' : 'false' }}
            </label>

            <input
              v-else-if="typeof field.value === 'number'"
              type="number"
              :id="'field-' + field.dottedKey.replace(/\./g, '-')"
              :data-key="field.dottedKey"
              v-model="formData[field.dottedKey]"
              class="form-input"
            />

            <div v-else>
              <input
                type="text"
                :id="'field-' + field.dottedKey.replace(/\./g, '-')"
                :data-key="field.dottedKey"
                v-model="formData[field.dottedKey]"
                class="form-input"
              />
              <div v-if="isDuration(field.dottedKey)" class="meta-text">format: number + s/m/h (e.g. 30s, 5m, 1h)</div>
            </div>

            <button
              v-if="field.isOverride"
              class="btn btn-danger text-2xs ml-2"
              @click="handleReset(field.dottedKey)"
            >
              ✕ reset
            </button>
          </div>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-accent" @click="handleSave">save</button>
        </div>
      </template>
    </div>
  </div>
</template>