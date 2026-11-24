import { describe, expect, it } from 'vitest'
import { parseTab } from './tabs'

describe('parseTab', () => {
  it('aceita as abas conhecidas e cai em categorias no resto', () => {
    expect(parseTab('gastos')).toBe('gastos')
    expect(parseTab('cortar')).toBe('cortar')
    expect(parseTab(null)).toBe('categorias')
    expect(parseTab('xyz')).toBe('categorias')
  })
})
