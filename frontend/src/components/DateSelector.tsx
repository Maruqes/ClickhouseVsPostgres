import { Slider } from '@/components/ui/slider'
import { dateAt, dateLabel, dayCount, type Dataset, type DateRange } from '@/lib/analytics'
export function DateSelector({ dataset, value, onChange, onCommit }: {
  dataset: Dataset; value: DateRange; onChange: (value: DateRange) => void; onCommit: (value: DateRange) => void
}) {
  const max = dayCount(dataset)
  return <section className="date-selector" aria-label="Date range">
    <div className="date-heading"><span className="eyebrow">Time frame · UTC</span><button className="text-button" onClick={() => { onChange([0, max]); onCommit([0, max]) }}>Full range</button></div>
    <div className="date-values"><time dateTime={dateAt(dataset, value[0])}>{dateLabel(dateAt(dataset, value[0]))}</time><span aria-hidden="true">—</span><time dateTime={dateAt(dataset, value[1])}>{dateLabel(dateAt(dataset, value[1]))}</time></div>
    <Slider min={0} max={max} step={1} value={value} thumbLabel={index => index === 0 ? 'Start date' : 'End date'} thumbValueText={offset => dateLabel(dateAt(dataset, offset))}
      onValueChange={next => onChange(next as DateRange)} onValueCommitted={next => onCommit(next as DateRange)} />
  </section>
}
