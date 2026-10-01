import { act, cleanup, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { useRequest, type Analytics } from './lib/analytics'
const dataset = { version: 'exercise-v2', rows: 10000000, from: '2023-01-01', to: '2025-12-31', muscles: ['back','shoulder','legs','chest','tricep','bicep','forearm'] }
const totals = { volume_kg: '480.25', athletes: 1, sets: 1, reps: 8 }
function result(country: string): Analytics {
  return { country, from: dataset.from, to: dataset.to, totals, query_ms: 1.25, muscles: dataset.muscles.map(key => ({key, volume_kg: '0.00'})), days: [{key:dataset.from,volume_kg:'480.25'}] }
}
function json(data: unknown) { return { ok: true, json: async () => data } as Response }
const fetchMock = vi.fn<typeof fetch>()
beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
  fetchMock.mockImplementation(async input => {
    const url = String(input)
    return json(url === '/api/dataset' ? dataset : result(new URL(url, 'http://local').searchParams.get('country')!))
  })
  // Base UI uses browser observer APIs for slider positioning.
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
})
afterEach(() => { cleanup(); vi.unstubAllGlobals(); fetchMock.mockReset() })
async function openMap() {
  render(<App />)
  await waitFor(() => expect(screen.getByRole('button', {name:'Portugal'}).getAttribute('aria-disabled')).toBe('false'))
}

describe('lazy country analytics', () => {
  it('loads metadata only and commits slider changes without analytics', async () => {
    await openMap()
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual(['/api/dataset'])
    const start = screen.getByLabelText('Start date')
    fireEvent.keyDown(start, {key:'ArrowRight'})
    fireEvent.keyUp(start, {key:'ArrowRight'})
    await waitFor(() => expect(start.getAttribute('aria-valuenow')).toBe('1'))
    expect(fetchMock).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button',{name:'Portugal'}))
    await screen.findAllByText('480.25')
    expect(String(fetchMock.mock.calls[1][0])).toContain('from=2023-01-02')
    expect(String(fetchMock.mock.calls[1][0])).toContain('/detail?')
  })
  it('starts a fresh summary on every country entry and clears it on range commit', async () => {
    await openMap()
    const portugal = screen.getByRole('button',{name:'Portugal'})
    fireEvent.pointerOver(portugal)
    await screen.findByText('480.25')
    fireEvent.pointerOut(portugal)
    fireEvent.pointerOver(portugal)
    await screen.findByText('480.25')
    expect(fetchMock.mock.calls.filter(([url]) => String(url).includes('/summary?'))).toHaveLength(2)
    const count = fetchMock.mock.calls.length
    const start = screen.getByLabelText('Start date')
    fireEvent.keyDown(start,{key:'ArrowRight'})
    fireEvent.keyUp(start,{key:'ArrowRight'})
    await waitFor(() => expect(screen.queryByText('480.25')).toBeNull())
    expect(fetchMock).toHaveBeenCalledTimes(count)
  })
  it('opens details on click, with all categories and an accessible daily table', async () => {
    await openMap()
    fireEvent.click(screen.getByRole('button',{name:'Portugal'}))
    await screen.findByText('Volume by muscle area')
    expect(screen.getAllByText('forearm').length).toBe(1)
    expect(screen.getByText('View daily values')).toBeTruthy()
    expect(screen.getByRole('button',{name:'Close'})).toBeTruthy()
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})

it('aborts superseded requests and rejects stale responses even if cancellation is ignored', async () => {
  let resolveOld!: (value: Response) => void
  fetchMock.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockResolvedValueOnce(json({country:'GB'}))
  const {result: hook, unmount} = renderHook(() => useRequest<{country:string}>())
  let old!: Promise<void>
  act(() => { old = hook.current.run('/old') })
  const oldSignal = fetchMock.mock.calls[0][1]!.signal!
  await act(() => hook.current.run('/new'))
  expect(oldSignal.aborted).toBe(true)
  await act(async () => { resolveOld(json({country:'US'})); await old })
  expect(hook.current.state).toEqual({status:'success',data:{country:'GB'}})
  unmount()
  expect(fetchMock.mock.calls[1][1]!.signal!.aborted).toBe(true)
})

it('shows failures and performs no automatic retry', async () => {
  fetchMock.mockResolvedValueOnce({ok:false,status:503} as Response)
  const {result:hook} = renderHook(() => useRequest())
  await act(() => hook.current.run('/api/country/summary'))
  expect(hook.current.state.status).toBe('error')
  expect(fetchMock).toHaveBeenCalledTimes(1)
})
