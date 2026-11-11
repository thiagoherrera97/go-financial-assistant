import { describe, expect, it } from 'vitest'
import { OTHER_SHARE_WARNING, formatShare, shouldWarnOtherShare } from './otherShare'

describe('shouldWarnOtherShare', () => {
  it('o limite é 30%', () => {
    expect(OTHER_SHARE_WARNING).toBe(0.3)
  })
  it('exatamente 30% não avisa; 30,1% avisa', () => {
    expect(shouldWarnOtherShare(0.3)).toBe(false)
    expect(shouldWarnOtherShare(0.301)).toBe(true)
  })
  it('não avisa abaixo do limite nem com a divisão imprecisa de 30%', () => {
    expect(shouldWarnOtherShare(0)).toBe(false)
    expect(shouldWarnOtherShare(0.29)).toBe(false)
    expect(shouldWarnOtherShare(0.1 + 0.1 + 0.1)).toBe(false) // 0,30000000000000004
  })
  it('avisa quando quase tudo está em Outros', () => {
    expect(shouldWarnOtherShare(0.91)).toBe(true)
    expect(shouldWarnOtherShare(1)).toBe(true)
  })
  it('mês sem despesas (null ou ausente) não avisa', () => {
    expect(shouldWarnOtherShare(null)).toBe(false)
    expect(shouldWarnOtherShare(undefined)).toBe(false)
  })
})

describe('formatShare', () => {
  it('mostra uma casa só quando precisa', () => {
    expect(formatShare(0.91)).toBe('91%')
    expect(formatShare(0.301)).toBe('30,1%')
    expect(formatShare(1)).toBe('100%')
  })
})
