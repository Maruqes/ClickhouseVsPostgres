import { useEffect, useState } from 'react'
import { ArrowUpRight } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { DateSelector } from '@/components/DateSelector'
import { WorldMap } from '@/components/WorldMap'
import { CountryDetails, type Country } from '@/components/CountryDetails'
import { Stats, CalculationTime } from '@/components/Stats'
import { analyticsURL, dayCount, useRequest, type Analytics, type Dataset, type DateRange } from '@/lib/analytics'
import { Classes } from '@/components/Classes'
import { AppHeader } from '@/components/AppHeader'
export default function App() {
 return window.location.pathname === '/classes' ? <Classes /> : <Atlas />
}
function Atlas() {
  const metadata = useRequest<Dataset>()
  const summary = useRequest<Analytics>()
  const detail = useRequest<Analytics>()
  const [draftRange, setDraftRange] = useState<DateRange | null>(null)
  const [committedRange, setCommittedRange] = useState<DateRange | null>(null)
  const [hovered, setHovered] = useState<Country | null>(null)
  const [selected, setSelected] = useState<Country | null>(null)
  const dataset = metadata.state.status === 'success' ? metadata.state.data : null
  const fullRange: DateRange = [0, dataset ? dayCount(dataset) : 0]
  const range = committedRange ?? fullRange
  const loadMetadata = metadata.run
  useEffect(() => { void loadMetadata('/api/dataset') }, [loadMetadata])
  function enter(country: Country) {
    if (!dataset || selected) return
    setHovered(country)
    void summary.run(analyticsURL('summary', country.code, dataset, range))
  }
  function leave() { setHovered(null); summary.cancel() }
  function select(country: Country) {
    if (!dataset) return
    leave()
    setSelected(country)
    void detail.run(analyticsURL('detail', country.code, dataset, range))
  }
  function commit(next: DateRange) {
    if (next[0] === range[0] && next[1] === range[1]) return
    setCommittedRange(next)
    leave()
    detail.cancel()
    setSelected(null)
  }
  function close() { detail.cancel(); setSelected(null) }
  return <main className="atlas-app">
    <AppHeader page="analytics">
      {dataset && <DateSelector dataset={dataset} value={draftRange ?? fullRange} onChange={setDraftRange} onCommit={commit} />}
    </AppHeader>
    <div className="page-intro"><div><p className="eyebrow">Global training activity</p><h1>Every set.<br /><span>A world of effort.</span></h1></div><p>Explore training volume across countries.<br />Choose a time frame, then a place.</p></div>
    {metadata.state.status === 'error' && <div className="metadata-error" role="alert"><p>{metadata.state.message}</p><button className="action-button" onClick={() => void loadMetadata('/api/dataset')}>Try again</button></div>}
    {metadata.state.status === 'loading' && <p className="metadata-loading" role="status">Connecting to the dataset…</p>}
    <div className="map-container"><WorldMap active={hovered ?? selected} disabled={!dataset} onEnter={enter} onLeave={leave} onSelect={select} />
      {hovered && <Card className="summary-card" aria-live="polite"><CardContent><div className="summary-heading"><div><p className="eyebrow">{hovered.code} · Athlete nationality</p><h2>{hovered.name}</h2></div><ArrowUpRight size={22} aria-hidden="true" /></div>
        {summary.state.status === 'loading' && <p className="request-loading" role="status"><span className="loader" />Calculating…</p>}
        {summary.state.status === 'error' && <p className="request-error" role="alert">{summary.state.message}</p>}
        {summary.state.status === 'success' && <><Stats totals={summary.state.data.totals} /><CalculationTime ms={summary.state.data.query_ms} /></>}
      </CardContent></Card>}
    </div>
    <footer className="app-footer"><p><span className="status-dot" />{dataset ? `${dataset.rows.toLocaleString('en-US')} generated exercise sets` : 'Generated exercise dataset'}</p><p>Volume = repetitions × total load · Nationality · UTC</p></footer>
    <CountryDetails country={selected} state={detail.state} onClose={close} onRetry={() => { if (selected) select(selected) }} />
  </main>
}
