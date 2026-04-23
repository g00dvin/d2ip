import { ref } from 'vue'

const open = ref(false)
const message = ref('')
const queue = ref<((val: boolean) => void)[]>([])

export function useConfirm() {
  function confirm(msg: string): Promise<boolean> {
    message.value = msg
    open.value = true
    return new Promise((resolve) => {
      queue.value.push(resolve)
    })
  }

  function accept() {
    open.value = false
    const resolve = queue.value.shift()
    resolve?.(true)
  }

  function reject() {
    open.value = false
    const resolve = queue.value.shift()
    resolve?.(false)
  }

  return { open, message, confirm, accept, reject }
}
