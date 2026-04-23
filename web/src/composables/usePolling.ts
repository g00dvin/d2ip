import { ref, onMounted, onUnmounted } from 'vue'

export function usePolling(fn: () => Promise<void>, intervalMs: number | (() => number)) {
  let timer: ReturnType<typeof setInterval> | null = null
  const loading = ref(false)
  const lastError = ref<Error | null>(null)

  function getInterval(): number {
    return typeof intervalMs === 'function' ? intervalMs() : intervalMs
  }

  async function poll() {
    loading.value = true
    try {
      await fn()
      lastError.value = null
    } catch (e) {
      lastError.value = e as Error
      console.error('Polling error:', e)
    } finally {
      loading.value = false
    }
  }

  function start() {
    stop()
    poll()
    timer = setInterval(poll, getInterval())
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  onMounted(start)
  onUnmounted(stop)

  return { loading, lastError, start, stop, poll }
}
