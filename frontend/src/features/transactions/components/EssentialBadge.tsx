import { essentialLabel } from '../../../shared/api/subcategories'
import type { Essential } from '../../../shared/api/subcategories'

/** Selo de essencial, supérfluo ou a revisar de uma despesa. */
export function EssentialBadge({ essential }: { essential: Essential }) {
  const tone = essential === null ? 'unknown' : essential ? 'essential' : 'superfluous'
  return <span className={`badge badge-${tone}`}>{essentialLabel(essential)}</span>
}
