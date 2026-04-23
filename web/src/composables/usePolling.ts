import { ref, onUnmounted, watch } from 'vue'

export function usePolling(fn: () => Promise<void>, intervalMs: number | (() => number), opts?: { immediate?: boolean }) {
  let timer: ReturnType<typeof setInterval> | null = null
  const loading = ref(false)

  function getInterval(): number {
    return typeof intervalMs === 'function' ? intervalMs() : intervalMs
  }

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
    timer = setInterval(() => poll(), getInterval())
  }

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  if (typeof intervalMs === 'function') {
    watch(intervalMs, () => {
      stop()
      start()
    })
  }

  onUnmounted(() => stop())

  start()

  return { loading, start, stop, poll }
}