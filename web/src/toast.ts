import { reactive } from 'vue'

export interface ToastItem {
  id: number
  msg: string
  type: 'error' | 'success' | 'info'
}

export const items = reactive<ToastItem[]>([])
let seq = 0

export function toast(msg: string, type: ToastItem['type'] = 'error') {
  const id = ++seq
  items.push({ id, msg, type })
  setTimeout(() => {
    const i = items.findIndex((t) => t.id === id)
    if (i >= 0) items.splice(i, 1)
  }, 3000)
}
