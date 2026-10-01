import { useState } from 'react'
import { dateLabel, decimalLabel, type Volume } from '@/lib/analytics'
export function VolumeChart({ days }: { days: Volume[] }) {
  const [inspection, setInspection] = useState<number | null>(null)
  const maximum = Math.max(1, ...days.map(day => Number(day.volume_kg)))
  const x = (i: number) => 50 + i / Math.max(1, days.length - 1) * 830
  const y = (volume: string) => 185 - Number(volume) / maximum * 145
  const points = days.map((day, i) => `${x(i)},${y(day.volume_kg)}`).join(' ')
  const selected = inspection === null ? null : days[inspection]
  const tick = (value: number) => new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 1 }).format(value)
  return <section className="daily-chart" aria-label="Daily training volume">
    <div className="section-heading"><h3>Volume over time</h3><span>Daily · kg</span></div>
    <svg viewBox="0 0 910 230" role="img" aria-label={`Daily training volume from ${days[0].key} to ${days[days.length - 1].key}`}>
      <title>Daily training volume. All UTC days are included, including days with no sets.</title>
      {[0, 0.5, 1].map(fraction => <g key={fraction}><line x1="50" x2="880" y1={185 - fraction * 145} y2={185 - fraction * 145} className="chart-grid" /><text x="42" y={189 - fraction * 145} textAnchor="end">{tick(maximum * fraction)}</text></g>)}
      <polygon points={`50,185 ${points} ${x(days.length - 1)},185`} className="chart-area" /><polyline points={points} className="chart-line" />
      {days.length === 1 && <circle cx={x(0)} cy={y(days[0].volume_kg)} r="3" className="chart-point" />}
      <text x="50" y="215">{dateLabel(days[0].key)}</text><text x="880" y="215" textAnchor="end">{dateLabel(days[days.length - 1].key)}</text>
      {selected && <g><line x1={x(inspection!)} x2={x(inspection!)} y1="35" y2="185" className="chart-cursor" /><circle cx={x(inspection!)} cy={y(selected.volume_kg)} r="4" className="chart-point" /></g>}
      <rect x="50" y="30" width="830" height="155" fill="transparent" onPointerMove={event => {
        const rect = event.currentTarget.getBoundingClientRect()
        setInspection(Math.max(0, Math.min(days.length - 1, Math.round((event.clientX - rect.left) / rect.width * (days.length - 1)))))
      }} onPointerLeave={() => setInspection(null)} />
    </svg>
    <div className="chart-inspection" aria-live="polite">{selected ? `${dateLabel(selected.key)} · ${decimalLabel(selected.volume_kg)} kg` : 'Move over the chart to inspect a day.'}</div>
    <details className="chart-data"><summary>View daily values</summary><div className="daily-table"><table><thead><tr><th scope="col">UTC day</th><th scope="col">Volume (kg)</th></tr></thead><tbody>{days.map(day => <tr key={day.key}><td>{day.key}</td><td>{decimalLabel(day.volume_kg)}</td></tr>)}</tbody></table></div></details>
  </section>
}
