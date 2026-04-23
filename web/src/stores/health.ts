import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '@/api/rest'

export const useHealthStore = defineStore('health', () => {
  const status = ref<'healthy' | 'unhealthy' | 'checking'>('checking')

  async function fetchHealth() {
    try {
      const data = await api.getHealth()
      status.value = data.status === 'ok' ? 'healthy' : 'unhealthy'
    } catch {
      status.value = 'unhealthy'
    }
  }

  return { status, fetchHealth }
})
