import { ref } from 'vue'

const open = ref(false)
const message = ref('')
let _resolve: ((val: boolean) => void) | null = null

export function useConfirm() {
  function confirm(msg: string): Promise<boolean> {
    message.value = msg
    open.value = true
    return new Promise((resolve) => {
      _resolve = resolve
    })
  }

  function accept() {
    open.value = false
    _resolve?.(true)
    _resolve = null
  }

  function reject() {
    open.value = false
    _resolve?.(false)
    _resolve = null
  }

  return { open, message, confirm, accept, reject }
}