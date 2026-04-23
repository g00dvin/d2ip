import { ref } from 'vue'
import { getSourceInfo } from '@/api/rest'
import type { SourceInfo } from '@/api/types'

export const info = ref<SourceInfo | null>(null)

export async function fetchSourceInfo() {
  try {
    info.value = await getSourceInfo()
  } catch {
    // keep previous state
  }
}