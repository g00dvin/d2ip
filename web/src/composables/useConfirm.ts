import { ref } from 'vue'

export function useConfirm() {
  const visible = ref(false)
  const message = ref('')
  let resolveFn: ((value: boolean) => void) | null = null

  function confirm(msg: string): Promise<boolean> {
    message.value = msg
    visible.value = true
    return new Promise((resolve) => {
      resolveFn = resolve
    })
  }

  function onOk() {
    visible.value = false
    resolveFn?.(true)
    resolveFn = null
  }

  function onCancel() {
    visible.value = false
    resolveFn?.(false)
    resolveFn = null
  }

  return { visible, message, confirm, onOk, onCancel }
}
