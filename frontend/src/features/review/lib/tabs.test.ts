import { describe, expect, it } from 'vitest'
import { parseTab } from './tabs'

describe('parseTab', () => {
  it('aceita as abas conhecidas e cai em cortar no resto', () => {
    expect(parseTab('gastos')).toBe('gastos')
    expect(parseTab('categorias')).toBe('categorias')
    expect(parseTab(null)).toBe('cortar')
    expect(parseTab('xyz')).toBe('cortar')
  })
})
