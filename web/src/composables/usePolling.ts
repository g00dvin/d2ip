import { ref, onUnmounted } from 'vue'

export function usePolling(fn: () => Promise<void>, intervalMs: number, opts?: { immediate?: boolean }) {
  let timer: ReturnType<typeof setInterval> | null = null
  const loading = ref(false)

  async function poll() {
    loading.value = true
    try {
      await fn()
    } finally {
      loading.value = false
    }
  }

  function start() {
    stop()
    if (opts?.immediate !== false) {
      poll()
    }
    timer = setInterval(() => poll(), intervalMs)
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  onUnmounted(() => stop())

  return { loading, start, stop, poll }
}