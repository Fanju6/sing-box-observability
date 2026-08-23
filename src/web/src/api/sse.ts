import type { EventEnvelope, EventType } from './types'

type EventListener = (event: EventEnvelope) => void
type StateListener = (state: number, lastSequence: number | null) => void

const READY_STATE = {
  CONNECTING: 0,
  OPEN: 1,
  CLOSED: 2,
} as const

const EVENT_TYPES: EventType[] = ['hello', 'source.state', 'connection.open', 'connection.close', 'resync']

export class SSEManager {
  private es: EventSource | null = null
  private listeners: Map<EventType, Set<EventListener>> = new Map()
  private stateListeners: Set<StateListener> = new Set()
  private lastSequence: number | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectDelay = 1000
  private maxReconnectDelay = 30000
  private intentionalClose = false
  private lastEventId: string | null = null

  connect() {
    if (this.es && (this.es.readyState === READY_STATE.CONNECTING || this.es.readyState === READY_STATE.OPEN)) {
      return
    }
    this.intentionalClose = false

    const url = new URL('/api/v1/events?heartbeat=15s', window.location.origin)
    if (this.lastEventId) {
      url.searchParams.set('lastEventId', this.lastEventId)
    }

    this.es = new EventSource(url.toString(), { withCredentials: true })
    this.notifyState()

    this.es.onopen = () => {
      this.reconnectDelay = 1000
      this.notifyState()
    }

    const handleEvent = (rawEvent: Event) => {
      const ev = rawEvent as MessageEvent<string>
      try {
        const data: EventEnvelope = JSON.parse(ev.data)
        this.lastSequence = data.sequence
        if (ev.lastEventId) this.lastEventId = ev.lastEventId
        this.notifyState()

        const typeListeners = this.listeners.get(data.type)
        if (typeListeners) {
          typeListeners.forEach((fn) => fn(data))
        }
        const allListeners = this.listeners.get('*' as EventType)
        if (allListeners) {
          allListeners.forEach((fn) => fn(data))
        }
      } catch {
        // malformed event, ignore
      }
    }
    EVENT_TYPES.forEach((type) => this.es?.addEventListener(type, handleEvent))

    this.es.onerror = () => {
      this.notifyState()
      this.es?.close()
      this.es = null
      if (!this.intentionalClose) {
        this.scheduleReconnect()
      }
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
      this.connect()
    }, this.reconnectDelay)
  }

  disconnect() {
    this.intentionalClose = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.es?.close()
    this.es = null
    this.notifyState()
  }

  on(type: EventType | '*', fn: EventListener) {
    const key = type as EventType
    if (!this.listeners.has(key)) {
      this.listeners.set(key, new Set())
    }
    this.listeners.get(key)!.add(fn)
    return () => this.off(key, fn)
  }

  off(type: EventType | '*', fn: EventListener) {
    const key = type as EventType
    this.listeners.get(key)?.delete(fn)
  }

  onStateChange(fn: StateListener) {
    this.stateListeners.add(fn)
    fn(this.getReadyState(), this.lastSequence)
    return () => this.stateListeners.delete(fn)
  }

  private notifyState() {
    const state = this.getReadyState()
    this.stateListeners.forEach((fn) => fn(state, this.lastSequence))
  }

  getReadyState(): number {
    return this.es?.readyState ?? READY_STATE.CLOSED
  }

  reset() {
    this.lastSequence = null
    this.lastEventId = null
    this.disconnect()
    this.connect()
  }

  isConnected(): boolean {
    return this.es?.readyState === READY_STATE.OPEN
  }
}

export const sseManager = new SSEManager()
export { READY_STATE as SSE_READY_STATE }
