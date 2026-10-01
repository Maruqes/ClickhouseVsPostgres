import { useCallback, useEffect, useRef, useState } from 'react'
export type Dataset = { version: string; rows: number; from: string; to: string; muscles: string[] }
export type DateRange = [number, number]
export type Volume = { key: string; volume_kg: string }
export type Totals = { volume_kg: string; athletes: number; sets: number; reps: number }
export type Analytics = { country: string; from: string; to: string; totals: Totals; muscles?: Volume[]; days?: Volume[]; query_ms: number }
export type RequestState<T> = { status: 'idle' } | { status: 'loading' } | { status: 'error'; message: string } | { status: 'success'; data: T }

export async function request<T>(path: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(path, { signal, cache: 'no-store', headers: { Accept: 'application/json' } })
  if (!response.ok) {
    if (response.status === 504) throw new Error('The calculation timed out. Try a shorter date range.')
    if (response.status === 503) throw new Error('The dataset is unavailable. Wait for startup to finish and try again.')
    throw new Error('The calculation failed. Try selecting the country again.')
  }
  return response.json() as Promise<T>
}

// Abort and generation checks protect against late responses even when transport
// cancellation races with completion. Every entry/click starts fresh database work.
export function useRequest<T>() {
  const [state, setState] = useState<RequestState<T>>({ status: 'idle' })
  const controller = useRef<AbortController | null>(null)
  const generation = useRef(0)
  const cancel = useCallback(() => {
    generation.current += 1
    controller.current?.abort()
    controller.current = null
    setState({ status: 'idle' })
  }, [])
  const run = useCallback(async (path: string) => {
    controller.current?.abort()
    const current = ++generation.current
    const active = new AbortController()
    controller.current = active
    setState({ status: 'loading' })
    try {
      const data = await request<T>(path, active.signal)
      if (current === generation.current && !active.signal.aborted) setState({ status: 'success', data })
    } catch (error) {
      if (current === generation.current && !active.signal.aborted) {
        setState({ status: 'error', message: error instanceof TypeError ? 'Connection lost. Check the backend and try again.' : error instanceof Error ? error.message : 'Request failed. Try again.' })
      }
    }
  }, [])
  useEffect(() => () => {
    generation.current += 1
    controller.current?.abort()
  }, [])
  return { state, run, cancel }
}

const DAY_MS = 86_400_000
export function dayCount(dataset: Dataset) { return (Date.parse(dataset.to) - Date.parse(dataset.from)) / DAY_MS }
export function dateAt(dataset: Dataset, offset: number) { return new Date(Date.parse(dataset.from) + offset * DAY_MS).toISOString().slice(0, 10) }
export function analyticsURL(kind: 'summary' | 'detail', country: string, dataset: Dataset, range: DateRange) {
  return `/api/country/${kind}?${new URLSearchParams({ country, from: dateAt(dataset, range[0]), to: dateAt(dataset, range[1]) })}`
}
export function decimalLabel(value: string) {
  const [whole, fraction = '00'] = value.split('.')
  return `${BigInt(whole).toLocaleString('en-US')}.${fraction.padEnd(2, '0')}`
}
export function dateLabel(value: string) {
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' }).format(new Date(value))
}
