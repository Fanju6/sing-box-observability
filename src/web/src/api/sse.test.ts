import { SSEManager } from './sse'
import type { EventEnvelope } from './types'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

class FakeEventSource {
  static instances: FakeEventSource[] = []
  readonly url: string
  readyState = 0
  onopen: ((event: Event) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  private listeners = new Map<string, Set<(event: Event) => void>>()

  constructor(url: string | URL) {
    this.url = String(url)
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    const fn = typeof listener === 'function' ? listener : listener.handleEvent.bind(listener)
    const listeners = this.listeners.get(type) ?? new Set()
    listeners.add(fn)
    this.listeners.set(type, listeners)
  }

  close() { this.readyState = 2 }

  open() {
    this.readyState = 1
    this.onopen?.(new Event('open'))
  }

  emit(type: string, envelope: EventEnvelope, id: string) {
    const event = new MessageEvent(type, { data: JSON.stringify(envelope), lastEventId: id })
    this.listeners.get(type)?.forEach((listener) => listener(event))
  }

  fail() {
    this.readyState = 2
    this.onerror?.(new Event('error'))
  }
}

describe('SSEManager', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeEventSource.instances = []
    vi.stubGlobal('EventSource', FakeEventSource)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('receives named SSE events and replays from the last event id', () => {
    const manager = new SSEManager()
    const listener = vi.fn()
    const stateListener = vi.fn()
    manager.on('connection.open', listener)
    manager.onStateChange(stateListener)
    manager.connect()

    const first = FakeEventSource.instances[0]
    first.open()
    const envelope: EventEnvelope = {
      sequence: 42,
      generatedAt: '2026-08-21T00:00:00Z',
      type: 'connection.open',
      data: {
        id: 'c1', state: 'active', network: 'tcp', inbound: 'tun', destinationPort: 443,
        outbound: 'direct', outboundType: 'direct', chain: ['direct'], startedAt: '2026-08-21T00:00:00Z',
        closedAt: null, durationSeconds: 1, upload: 1, download: 2,
      },
    }
    first.emit('connection.open', envelope, '42')
    expect(listener).toHaveBeenCalledWith(envelope)
    expect(stateListener).toHaveBeenLastCalledWith(1, 42)

    first.fail()
    vi.advanceTimersByTime(1_000)
    expect(FakeEventSource.instances).toHaveLength(2)
    expect(new URL(FakeEventSource.instances[1].url).searchParams.get('lastEventId')).toBe('42')
    manager.disconnect()
  })
})
