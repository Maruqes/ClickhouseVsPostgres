import type { ReactNode } from 'react'
import { Bike, Globe2 } from 'lucide-react'

type Page = 'analytics' | 'classes'

export function AppHeader({ page, children }: { page: Page; children?: ReactNode }) {
  const Icon = page === 'classes' ? Bike : Globe2
  return <header className="app-header">
    <div className="brand-navigation">
      <a className="brand" href="/" aria-label="Exercise Atlas home"><span className="brand-icon"><Icon size={25} strokeWidth={1.5} /></span><span>Exercise Atlas<small>{page === 'classes' ? 'Reservation experiment' : "Explore the world's training"}</small></span></a>
      <nav className="page-nav" aria-label="Pages"><a href="/" aria-current={page === 'analytics' ? 'page' : undefined}>Exercise analytics</a><a href="/classes" aria-current={page === 'classes' ? 'page' : undefined}>Reservations</a></nav>
    </div>
    {children}
  </header>
}
