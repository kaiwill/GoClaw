import type { WsMessage } from '../types/api'
import { getToken } from './auth'

export type WsMessageHandler = (msg: WsMessage) => void
export type WsOpenHandler = () => void
export type WsCloseHandler = (ev: CloseEvent) => void
export type WsErrorHandler = (ev: Event) => void

export interface WebSocketClientOptions {
  baseUrl?: string
  reconnectDelay?: number
  maxReconnectDelay?: number
  autoReconnect?: boolean
}

const DEFAULT_RECONNECT_DELAY = 1000
const MAX_RECONNECT_DELAY = 30000

export class WebSocketClient {
  private ws: WebSocket | null = null
  private currentDelay: number
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private intentionallyClosed = false

  public onMessage: WsMessageHandler | null = null
  public onOpen: WsOpenHandler | null = null
  public onClose: WsCloseHandler | null = null
  public onError: WsErrorHandler | null = null

  private readonly baseUrl: string
  private readonly reconnectDelay: number
  private readonly maxReconnectDelay: number
  private readonly autoReconnect: boolean

  constructor(options: WebSocketClientOptions = {}) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // 直接连接到后端服务器，不通过Vite代理
    const host = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
      ? 'localhost:4096'
      : window.location.host
    this.baseUrl = options.baseUrl ?? `${protocol}//${host}`
    this.reconnectDelay = options.reconnectDelay ?? DEFAULT_RECONNECT_DELAY
    this.maxReconnectDelay = options.maxReconnectDelay ?? MAX_RECONNECT_DELAY
    this.autoReconnect = options.autoReconnect ?? true
    this.currentDelay = this.reconnectDelay
  }

  connect(): void {
    this.intentionallyClosed = false
    this.clearReconnectTimer()

    const token = getToken()
    let url = `${this.baseUrl}/api/ws/chat`
    const protocols = ['zeroclaw.v1']
    if (token) {
      // 将token放在查询参数中
      url += `?token=${encodeURIComponent(token)}`
    }

    console.log('WebSocket connecting to:', url)

    this.ws = new WebSocket(url, protocols)

    this.ws.onopen = () => {
      console.log('WebSocket connected')
      this.currentDelay = this.reconnectDelay
      this.onOpen?.()
    }

    this.ws.onmessage = (ev: MessageEvent) => {
      try {
        const msg = JSON.parse(ev.data) as WsMessage
        this.onMessage?.(msg)
      } catch {
        console.error('Failed to parse WebSocket message:', ev.data)
      }
    }

    this.ws.onclose = (ev: CloseEvent) => {
      console.log('WebSocket closed:', ev.code, ev.reason)
      this.onClose?.(ev)
      this.scheduleReconnect()
    }

    this.ws.onerror = (ev: Event) => {
      console.error('WebSocket error:', ev)
      this.onError?.(ev)
    }
  }

  sendMessage(content: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket is not connected')
    }
    this.ws.send(JSON.stringify({ type: 'message', content: content }))
  }

  disconnect(): void {
    this.intentionallyClosed = true
    this.clearReconnectTimer()
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN
  }

  private scheduleReconnect(): void {
    if (this.intentionallyClosed || !this.autoReconnect) return

    this.reconnectTimer = setTimeout(() => {
      this.currentDelay = Math.min(this.currentDelay * 2, this.maxReconnectDelay)
      this.connect()
    }, this.currentDelay)
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }
}
