/**
 * WebSocket 客户端：心跳 + 指数退避自动重连 + 事件分发
 */

export type WSEvent = string
export type Handler = (data: any) => void

export interface WSOptions {
  token: string
  /** 管理端需指定 quiz id */
  quiz?: number
  onEvent: Handler
  onStatus?: (status: 'connecting' | 'open' | 'closed' | 'retrying') => void
}

/** token 走 Sec-WebSocket-Protocol 子协议（不下发 URL，避免进访问日志）；后端从该头读取 */
export function wsURL(quiz?: number) {
  const base = import.meta.env.VITE_WS_BASE || `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}`
  return quiz ? `${base}/ws?quiz=${quiz}` : `${base}/ws`
}

export class QuizWS {
  private opts: WSOptions
  private ws: WebSocket | null = null
  private retries = 0
  private heartbeatTimer: number | null = null
  private reconnectTimer: number | null = null
  private closedByUser = false

  constructor(opts: WSOptions) {
    this.opts = opts
    this.connect()
  }

  private connect() {
    this.opts.onStatus?.('connecting')
    const ws = new WebSocket(wsURL(this.opts.quiz), [this.opts.token])
    this.ws = ws

    ws.onopen = () => {
      this.retries = 0
      this.opts.onStatus?.('open')
      this.startHeartbeat()
    }

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        if (msg && typeof msg.event === 'string') {
          this.opts.onEvent(msg)
        }
      } catch {
        /* ignore malformed */
      }
    }

    ws.onclose = () => {
      this.stopHeartbeat()
      if (this.closedByUser) {
        this.opts.onStatus?.('closed')
        return
      }
      this.scheduleReconnect()
    }

    ws.onerror = () => {
      // onclose 会跟着触发，统一在 onclose 处理重连
    }
  }

  private scheduleReconnect() {
    // 指数退避：1s, 2s, 4s, 8s ... 上限 15s
    const delay = Math.min(1000 * Math.pow(2, this.retries), 15000)
    this.retries++
    this.opts.onStatus?.('retrying')
    this.reconnectTimer = window.setTimeout(() => this.connect(), delay)
  }

  private startHeartbeat() {
    this.stopHeartbeat()
    this.heartbeatTimer = window.setInterval(() => {
      this.send('ping')
    }, 20000)
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  send(event: string, data?: unknown) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ event, data }))
    }
  }

  close() {
    this.closedByUser = true
    this.stopHeartbeat()
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
  }
}
