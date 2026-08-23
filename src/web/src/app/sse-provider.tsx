import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { sseManager } from '@/api/sse'
import { queryKeys } from '@/api/query-keys'
import type { EventEnvelope, SourceStateEventData, ResyncEventData } from '@/api/types'

export function SSEProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient()

  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        qc.invalidateQueries({ queryKey: queryKeys.meta })
        sseManager.connect()
      }
    }

    const handleOnline = () => {
      qc.invalidateQueries()
      sseManager.connect()
    }

    const unsubSource = sseManager.on('source.state', (event: EventEnvelope) => {
      const data = event.data as SourceStateEventData
      qc.setQueryData(queryKeys.meta, (old: unknown) => {
        if (!old || typeof old !== 'object') return old
        return { ...(old as Record<string, unknown>), source: { ...(old as { source: unknown }).source as Record<string, unknown>, state: data.state, lastSuccessAt: data.lastSuccessAt, lastErrorCode: data.errorCode } }
      })
    })

    const unsubOpen = sseManager.on('connection.open', () => {
      qc.invalidateQueries({ queryKey: ['connections', 'active'] })
    })

    const unsubClose = sseManager.on('connection.close', () => {
      qc.invalidateQueries({ queryKey: ['connections'] })
      qc.invalidateQueries({ queryKey: ['rankings'] })
    })

    const unsubResync = sseManager.on('resync', (_event: EventEnvelope) => {
      const data = _event.data as ResyncEventData
      void data
      qc.invalidateQueries({ queryKey: queryKeys.meta })
      qc.invalidateQueries({ queryKey: ['overview'] })
      qc.invalidateQueries({ queryKey: ['connections'] })
      qc.invalidateQueries({ queryKey: ['rankings'] })
    })

    const unsubHello = sseManager.on('hello', () => {
      // connection established, refresh data
    })

    sseManager.connect()
    document.addEventListener('visibilitychange', handleVisibility)
    window.addEventListener('online', handleOnline)

    return () => {
      unsubSource()
      unsubOpen()
      unsubClose()
      unsubResync()
      unsubHello()
      document.removeEventListener('visibilitychange', handleVisibility)
      window.removeEventListener('online', handleOnline)
      sseManager.disconnect()
    }
  }, [qc])

  return <>{children}</>
}
