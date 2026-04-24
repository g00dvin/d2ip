import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'

export const useVersionStore = defineStore('version', () => {
  const version = ref<string>('')
  const buildTime = ref<string>('')

  async function fetchVersion() {
    try {
      const data = await api.getVersion()
      version.value = data.version
      buildTime.value = data.build_time
    } catch {
      // silently ignore; version is non-critical
    }
  }

  return { version, buildTime, fetchVersion }
})
