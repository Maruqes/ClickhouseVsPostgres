import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Stats, CalculationTime } from '@/components/Stats'
import { VolumeChart } from '@/components/VolumeChart'
import { dateLabel, decimalLabel, type Analytics, type RequestState } from '@/lib/analytics'
export type Country = { code: string; name: string; path: string }
export function CountryDetails({ country, state, onClose, onRetry }: {
  country: Country | null; state: RequestState<Analytics>; onClose: () => void; onRetry: () => void
}) {
  const data = state.status === 'success' ? state.data : null
  const maxVolume = Math.max(1, ...(data?.muscles ?? []).map(item => Number(item.volume_kg)))
  return <Dialog open={country !== null} onOpenChange={open => { if (!open) onClose() }}>
    <DialogContent className="country-dialog sm:max-w-4xl"><DialogHeader>
      <p className="eyebrow">Country detail · {country?.code}</p><DialogTitle className="country-title">{country?.name}</DialogTitle>
      <DialogDescription>{data ? `${dateLabel(data.from)} — ${dateLabel(data.to)} · Athlete nationality` : 'Training volume, athletes and completed exercise sets.'}</DialogDescription>
    </DialogHeader>
      {state.status === 'loading' && <div className="request-loading" role="status"><span className="loader" />Calculating country statistics…</div>}
      {state.status === 'error' && <div className="request-error" role="alert"><p>{state.message}</p><button className="action-button" onClick={onRetry}>Try again</button></div>}
      {data && <><Stats totals={data.totals} />
        {data.totals.sets === 0 && <p className="empty-note">No exercise sets in this time frame. Try another country or a wider range.</p>}
        <VolumeChart key={`${data.from}/${data.to}`} days={data.days!} />
        <section className="muscle-section"><div className="section-heading"><h3>Volume by muscle area</h3><span>kg</span></div><div className="muscle-grid">
          {data.muscles!.map(muscle => <div className="muscle-row" key={muscle.key}><div className="muscle-label"><span>{muscle.key}</span><strong>{decimalLabel(muscle.volume_kg)}</strong></div><div className="muscle-track"><div style={{ width: `${Number(muscle.volume_kg) / maxVolume * 100}%` }} /></div></div>)}
        </div></section><CalculationTime ms={data.query_ms} /></>}
    </DialogContent>
  </Dialog>
}
