import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { Classes } from './Classes'

const template = { id: 'template', name: 'Spinning', capacity: 20, reservations: Array.from({ length: 19 }, (_, n) => ({ id: `initial-${n}` })) }
function report(accepted = 1) {
  return {
    fixture_id: 'new-run', name: 'Spinning', capacity: 20, initial_bookings: 19, final_bookings: 19 + accepted, elapsed_ms: 32.4,
    summary: { accepted, rejected: 20 - accepted, errors: 0, infrastructure_errors: 0, overbooked: Math.max(accepted - 1, 0) },
    attempts: Array.from({ length: 20 }, (_, n) => ({ number: n + 1, outcome: n < accepted ? 'accepted' : 'rejected', duration_ms: 12.3, reason: n < accepted ? '' : 'class_full', infrastructure_error: false })),
  }
}
function json(data: unknown) { return { ok: true, json: async () => data } as Response }
const fetchMock = vi.fn<typeof fetch>()
beforeEach(() => { vi.stubGlobal('fetch', fetchMock); fetchMock.mockResolvedValueOnce(json(template)) })
afterEach(() => { cleanup(); vi.unstubAllGlobals(); fetchMock.mockReset() })

it('loads the template, disables while running and replaces results with a fresh POST', async () => {
  let finish!: (value: Response) => void
  fetchMock.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  render(<Classes />)
  const button = screen.getByRole('button', { name: 'Try 20 reservations' }) as HTMLButtonElement
  await waitFor(() => expect(button.disabled).toBe(false))
  expect(screen.getAllByRole('listitem')).toHaveLength(20)
  expect(screen.getByText('19 booked / 20 seats · 1 free')).toBeTruthy()
  fireEvent.click(button)
  expect(button.disabled).toBe(true)
  fireEvent.click(button)
  expect(fetchMock).toHaveBeenCalledTimes(2)
  expect(fetchMock.mock.calls[1][0]).toBe('/api/classes/demo/race')
  expect(fetchMock.mock.calls[1][1]?.method).toBe('POST')
  await act(async () => { finish(json(report(3))) })
  expect(screen.getAllByRole('row')).toHaveLength(21)
  expect(screen.getByText('+2 reservations beyond capacity')).toBeTruthy()
  expect(screen.getByText('22 booked / 20 seats · 0 free')).toBeTruthy()
  fetchMock.mockResolvedValueOnce(json(report()))
  fireEvent.click(button)
  expect(screen.queryByRole('table')).toBeNull()
  expect(screen.getByText('19 booked / 20 seats · 1 free')).toBeTruthy()
  await screen.findByRole('table')
  expect(screen.queryByText('+2 reservations beyond capacity')).toBeNull()
  expect(fetchMock).toHaveBeenCalledTimes(3)
})
it('shows POST errors without automatically retrying and permits an explicit new run', async () => {
  fetchMock.mockRejectedValueOnce(new TypeError('network failed'))
  render(<Classes />)
  const button = screen.getByRole('button', { name: 'Try 20 reservations' }) as HTMLButtonElement
  await waitFor(() => expect(button.disabled).toBe(false))
  fireEvent.click(button)
  expect((await screen.findByRole('alert')).textContent).toContain('may have saved reservations')
  expect(fetchMock).toHaveBeenCalledTimes(2)
  expect(button.disabled).toBe(false)
  fetchMock.mockResolvedValueOnce(json(report()))
  fireEvent.click(button)
  await screen.findByRole('table')
  expect(screen.queryByRole('alert')).toBeNull()
})
it('cancels a disconnected page and ignores the late race response', async () => {
  let finish!: (value: Response) => void
  fetchMock.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  const { unmount } = render(<Classes />)
  const button = screen.getByRole('button', { name: 'Try 20 reservations' }) as HTMLButtonElement
  await waitFor(() => expect(button.disabled).toBe(false))
  fireEvent.click(button)
  const signal = fetchMock.mock.calls[1][1]!.signal!
  unmount()
  expect(signal.aborted).toBe(true)
  await act(async () => { finish(json(report())) })
  expect(fetchMock).toHaveBeenCalledTimes(2)
})
