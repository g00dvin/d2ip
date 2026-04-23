import { ref, watch, onUnmounted } from 'vue'
import { useEventSource } from '@vueuse/core'

export type SSEHandler = (data: unknown) => void

export function useSSE(handlers: Record<string, SSEHandler>) {
  const connected = ref(false)
  const error = ref<Error | null>(null)

  const { status, close, event, data } = useEventSource('/events', [], {
    immediate: true,
    autoReconnect: {
      retries: 5,
      delay: 3000,
      onFailed() {
        console.warn('SSE: max reconnect retries reached, falling back to polling')
      },
    },
  })

  watch(status, (s) => {
    connected.value = s === 'OPEN'
    if (s === 'OPEN') {
      error.value = null
    }
  }, { immediate: true })

  watch(event, (evName) => {
    const raw = data.value
    const eventType = evName ?? 'message'
    const handler = handlers[eventType]
    if (!handler) return

    if (typeof raw === 'string') {
      try {
        handler(JSON.parse(raw))
      } catch {
        handler(raw)
      }
    } else {
      handler(raw)
    }
  })

  onUnmounted(close)

  return { connected, error, status }
}
