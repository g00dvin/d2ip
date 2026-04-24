import { ref, watch, onUnmounted } from 'vue'
import { useEventSource } from '@vueuse/core'

export type SSEHandler = (data: unknown) => void

export function useSSE(handlers: Record<string, SSEHandler>) {
  const connected = ref(false)
  const error = ref<Error | null>(null)

  const { status, close, event, data, error: esError } = useEventSource('/events', [], {
    immediate: true,
    autoReconnect: {
      retries: 10,
      delay: 5000,
      onFailed() {
        console.warn('SSE: max reconnect retries reached, falling back to polling')
      },
    },
  })

  // Suppress expected SSE errors (server timeouts, client disconnects)
  // to avoid console noise. EventSource auto-reconnects automatically.
  watch(esError, (evt) => {
    if (evt) {
      // Only log once to avoid spamming the console
      error.value = new Error('SSE connection error')
    }
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
