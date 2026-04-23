import { ref } from 'vue'
import api, { type SourceInfo } from '@/api'

export const info = ref<SourceInfo | null>(null)

export async function fetchSourceInfo() {
  try {
    const { data } = await api.get<SourceInfo>('/api/source/info')
    info.value = data
  } catch {
    // keep previous state
  }
}