import { useEffect, useState } from 'react'
import { SSE_READY_STATE, sseManager } from '@/api/sse'
import type { SourceState } from '@/api/types'

export function useSSEState() {
  const [snapshot, setSnapshot] = useState(() => ({
    state: readyStateToSourceState(sseManager.getReadyState()),
    lastSequence: null as number | null,
  }))

  useEffect(() => {
    const unsubscribe = sseManager.onStateChange((readyState, lastSequence) => {
      setSnapshot({ state: readyStateToSourceState(readyState), lastSequence })
    })
    return () => { unsubscribe() }
  }, [])

  return snapshot
}

function readyStateToSourceState(readyState: number): SourceState {
  if (readyState === SSE_READY_STATE.OPEN) return 'online'
  if (readyState === SSE_READY_STATE.CONNECTING) return 'connecting'
  return 'offline'
}
