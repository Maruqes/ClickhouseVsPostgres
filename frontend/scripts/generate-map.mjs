// Generated public-domain Natural Earth 5.1.2 geometry; no runtime map service.
import { writeFile } from 'node:fs/promises'
import { geoNaturalEarth1, geoPath } from 'd3-geo'
const source = 'https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/geojson/ne_110m_admin_0_countries.geojson'
const response = await fetch(source)
if (!response.ok) throw new Error(`Map download failed: ${response.status}`)
const collection = await response.json()
const features = collection.features.filter(f => f.properties.ISO_A2_EH !== 'AQ')
const projection = geoNaturalEarth1().fitExtent([[20, 20], [980, 490]], { type: 'FeatureCollection', features })
const path = geoPath(projection).digits(2)
const aliases = { Somaliland: 'SO', 'Turkish Republic of Northern Cyprus': 'CY', Kosovo: 'XK' }
const merged = new Map()
for (const feature of features) {
  const properties = feature.properties
  const code = aliases[properties.NAME_EN] ?? properties.ISO_A2_EH
  if (!/^[A-Z]{2}$/.test(code)) throw new Error(`Missing code: ${properties.NAME_EN}`)
  const country = merged.get(code) ?? { code, name: properties.NAME_EN || properties.NAME, path: '' }
  country.path += path(feature)
  if (!aliases[properties.NAME_EN] || code === 'XK') country.name = properties.NAME_EN || properties.NAME
  merged.set(code, country)
}
const countries = [...merged.values()].sort((a, b) => a.name.localeCompare(b.name, 'en'))
await writeFile(new URL('../src/data/world.json', import.meta.url), JSON.stringify(countries) + '\n')
console.log(`Generated ${countries.length} country paths from Natural Earth 5.1.2`)
