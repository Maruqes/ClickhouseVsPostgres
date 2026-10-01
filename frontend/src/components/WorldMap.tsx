import { useState } from 'react'
import { Search, MousePointer2 } from 'lucide-react'
import countries from '@/data/world.json'
import { type Country } from '@/components/CountryDetails'
export function WorldMap({ active, disabled, onEnter, onLeave, onSelect }: {
  active: Country | null; disabled: boolean; onEnter: (country: Country) => void; onLeave: () => void; onSelect: (country: Country) => void
}) {
  const [search, setSearch] = useState('')
  const [searchOpen, setSearchOpen] = useState(false)
  const matches = countries.filter(country => `${country.name} ${country.code}`.toLowerCase().includes(search.toLowerCase()))
  return <section className="map-stage" aria-label="World exercise map">
    <div className="map-search"><label className="search-input"><Search size={16} aria-hidden="true" /><input aria-label="Find a country" value={search} placeholder="Find a country" onFocus={() => setSearchOpen(true)} onChange={event => { setSearch(event.target.value); setSearchOpen(true) }} onKeyDown={event => { if (event.key === 'Escape') setSearchOpen(false) }} /></label>
      {searchOpen && <div className="search-results" onBlur={event => { if (!event.currentTarget.parentElement?.contains(event.relatedTarget)) setSearchOpen(false) }}>
        <button className="search-dismiss" onClick={() => setSearchOpen(false)}>Close country list</button>
        {matches.map(country => <button key={country.code} disabled={disabled} onClick={() => { onSelect(country); setSearchOpen(false); setSearch('') }}>{country.name}<span>{country.code}</span></button>)}
        {matches.length === 0 && <p>No countries match. Try another name.</p>}
      </div>}
    </div>
    <svg className="world-map" viewBox="0 0 1000 510" aria-label="Select a country to explore exercise statistics" role="group">
      <defs><pattern id="map-grid" width="50" height="50" patternUnits="userSpaceOnUse"><path d="M 50 0 L 0 0 0 50" fill="none" stroke="currentColor" strokeWidth="0.4" /></pattern></defs><rect width="1000" height="510" fill="url(#map-grid)" className="map-grid" />
      {countries.map(country => <path key={country.code} d={country.path} className={`country-path ${active?.code === country.code ? 'active' : ''}`} role="button" aria-label={country.name} aria-disabled={disabled} tabIndex={disabled ? -1 : 0}
        onPointerEnter={event => { if (!disabled && event.pointerType !== 'touch') onEnter(country) }} onPointerLeave={onLeave}
        onFocus={() => { if (!disabled) onEnter(country) }} onBlur={onLeave} onClick={() => { if (!disabled) onSelect(country) }}
        onKeyDown={event => { if (!disabled && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); onSelect(country) } }}><title>{country.name}</title></path>)}
    </svg>
    <p className="map-instruction"><MousePointer2 size={15} aria-hidden="true" />Hover for a summary. Click to explore a country.</p>
    <span className="map-coordinate" aria-hidden="true">90° N<br />0°<br />90° S</span>
  </section>
}
