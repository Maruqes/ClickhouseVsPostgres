import { decimalLabel, type Totals } from '@/lib/analytics'
export function Stats({ totals }: { totals: Totals }) {
  return <dl className="stats-grid">
    <div className="volume-stat"><dt>Training volume</dt><dd>{decimalLabel(totals.volume_kg)} <span>kg</span></dd></div>
    <div><dt>Athletes</dt><dd>{totals.athletes.toLocaleString('en-US')}</dd></div>
    <div><dt>Completed sets</dt><dd>{totals.sets.toLocaleString('en-US')}</dd></div>
    <div><dt>Repetitions</dt><dd>{totals.reps.toLocaleString('en-US')}</dd></div>
  </dl>
}
export function CalculationTime({ ms }: { ms: number }) {
  return <p className="calculation-time"><span className="status-dot" />Calculated in {ms.toLocaleString('en-US', { maximumFractionDigits: 2 })} ms</p>
}
