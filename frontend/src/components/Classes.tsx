import { useCallback, useEffect, useRef, useState } from 'react'
import { AppHeader } from '@/components/AppHeader'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

type DemoClass = { id: string; name: string; capacity: number; reservations: { id: string }[] }
type Attempt = { number: number; outcome: 'accepted' | 'rejected' | 'error'; duration_ms: number; reason: string; infrastructure_error: boolean }
type Report = {
  fixture_id: string; name: string; capacity: number; initial_bookings: number; final_bookings: number; elapsed_ms: number
  attempts: Attempt[]
  summary: { accepted: number; rejected: number; errors: number; infrastructure_errors: number; overbooked: number }
}
const reasons: Record<string, string> = {
  class_full: 'Class full', reservation_unavailable: 'Reservation unavailable', write_uncertain: 'Write outcome uncertain',
  confirmed_from_storage: 'Confirmed in storage after an error',
}

export function Classes() {
  const [template, setTemplate] = useState<DemoClass | null>(null)
  const [report, setReport] = useState<Report | null>(null)
  const [loading, setLoading] = useState(true)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const controller = useRef<AbortController | null>(null)
  const busy = useRef(false)
  const load = useCallback(async (race: boolean) => {
    if (busy.current) return
    busy.current = true
    const active = new AbortController()
    controller.current = active
    if (race) setError(null)
    if (race) { setReport(null); setRunning(true) }
    // A disconnected POST is never retried: its committed fixture stays stored.
    const timeout = window.setTimeout(() => active.abort(), 35_000)
    try {
      const response = await fetch(race ? '/api/classes/demo/race' : '/api/classes/demo', {
        method: race ? 'POST' : 'GET', signal: active.signal, cache: 'no-store', headers: { Accept: 'application/json' },
      })
      if (!response.ok) throw new Error(response.status === 504 ? 'The experiment timed out.' : 'The reservation demo is unavailable.')
      const data = await response.json()
      if (!active.signal.aborted) { if (race) setReport(data as Report); else setTemplate(data as DemoClass) }
    } catch (cause) {
      if (controller.current === active) setError(`${cause instanceof Error && cause.name !== 'AbortError' && !(cause instanceof TypeError) ? cause.message : 'Connection lost or timed out.'} ${race ? 'The run may have saved reservations. Start a new isolated experiment when ready.' : 'Wait for startup to finish, then reload this page.'}`)
    } finally {
      window.clearTimeout(timeout)
      if (controller.current === active) { busy.current = false; setLoading(false); setRunning(false) }
    }
  }, [])
  useEffect(() => {
    // Fetching the template synchronizes the page with the database.
    // oxlint-disable-next-line react/set-state-in-effect
    void load(false)
    return () => { const active = controller.current; controller.current = null; busy.current = false; active?.abort() }
  }, [load])
  const capacity = report?.capacity ?? template?.capacity ?? 20
  const booked = report?.final_bookings ?? template?.reservations.length ?? 19
  return <main className="atlas-app classes-app">
    <AppHeader page="classes" />
    <div className="page-intro"><div><p className="eyebrow">One class · Twenty contenders</p><h1>The last seat.</h1></div><p>20 reservation attempts.<br />One place left when each run starts.</p></div>
    <Card className="class-card"><CardContent>
      <div className="class-heading"><h2>{report?.name ?? template?.name ?? 'Spinning'}</h2><p aria-live="polite">{booked} booked / {capacity} seats · {Math.max(capacity - booked, 0)} free</p></div>
      <ol className="seat-grid" aria-label={`${capacity} seats, ${Math.min(booked, capacity)} occupied`}>
        {Array.from({ length: capacity }, (_, index) => <li key={index} className={index < booked ? 'seat booked' : 'seat free'} aria-label={`Seat ${index + 1}: ${index < booked ? 'booked' : 'free'}`}>{String(index + 1).padStart(2, '0')}</li>)}
      </ol>
      {booked > capacity && <p className="overflow-note" role="status">+{booked - capacity} reservations beyond capacity</p>}
      <div className="class-action"><Button onClick={() => void load(true)} disabled={loading || running || !template}>Try 20 reservations</Button><p>{running ? 'Running 20 independent attempts…' : 'Every click starts fresh with 19 bookings.'}</p></div>
      {loading && <p role="status">Loading the class…</p>}
      {running && <p role="status" className="request-loading"><span className="loader" />Waiting for the completed results…</p>}
      {error && <p role="alert" className="request-error">{error}</p>}
    </CardContent></Card>
    {report && <section className="race-results" aria-label="Reservation results">
      <div className="race-totals" aria-live="polite"><span><strong>{report.summary.accepted}</strong> accepted</span><span><strong>{report.summary.rejected}</strong> rejected</span><span><strong>{report.summary.overbooked}</strong> overbooked seats</span><span><strong>{report.summary.errors}</strong> errors</span></div>
      <p className="race-duration">{report.elapsed_ms.toFixed(2)} ms total · {report.summary.infrastructure_errors} infrastructure errors{report.summary.infrastructure_errors > report.summary.errors ? ' (includes writes confirmed from storage)' : ''}</p>
      <div className="race-table"><table><caption>All 20 attempts, ordered by attempt number</caption><thead><tr><th scope="col">Attempt</th><th scope="col">Outcome</th><th scope="col">Time</th><th scope="col">Reason</th></tr></thead><tbody>{report.attempts.map(attempt => <tr key={attempt.number}><th scope="row">{String(attempt.number).padStart(2, '0')}</th><td><span className={`outcome ${attempt.outcome}`}>{attempt.outcome}</span></td><td>{attempt.duration_ms.toFixed(2)} ms</td><td>{reasons[attempt.reason] ?? (attempt.reason || 'Confirmed')}</td></tr>)}</tbody></table></div>
    </section>}
    <p className="race-explanation">PostgreSQL protects the last seat with a transaction and a class-row lock. This ClickHouse demo deliberately checks availability and then inserts without a lock; concurrent attempts can overbook. The actual result varies with timing. These are the booking strategies used here, not a guarantee about every application.</p>
  </main>
}
