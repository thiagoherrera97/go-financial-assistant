import { describe, expect, it } from 'vitest'
import { emptyVendorForm, formatDate, isOutflow, linkedIds, parseAmount, planStatus, toVendorInput } from './vendors'
import type { TransactionHit, Vendor, VendorSummary } from '../api'

const vendor = (over: Partial<Vendor>): Vendor => ({
  id: 'v',
  name: 'Buffet',
  contracted: 10000,
  dueDate: '2027-08-23',
  paid: 2000,
  remaining: 8000,
  done: false,
  contributions: 9,
  late: false,
  payments: [],
  ...over,
})

const summary = (over: Partial<VendorSummary>): VendorSummary => ({
  plan: { name: 'Casamento', contributionDay: 10, savingsBox: '', savedManual: 0 },
  boxes: [],
  contracted: 10000,
  paid: 2000,
  saved: 0,
  savedFromBox: false,
  remaining: 8000,
  nextContribution: '2026-12-10',
  lastDue: '2027-08-23',
  monthsLeft: 9,
  perMonth: 888.89,
  flatPerMonth: 888.89,
  tight: null,
  surplus: null,
  fits: null,
  vendors: [vendor({})],
  ...over,
})

describe('formatação', () => {
  it('data dd/mm/aaaa', () => expect(formatDate('2027-08-23')).toBe('23/08/2027'))
  it('valores no formato brasileiro ou com ponto', () => {
    expect(parseAmount('18.489,98')).toBe(18489.98)
    expect(parseAmount('R$ 4000')).toBe(4000)
    expect(parseAmount('3895.5')).toBe(3895.5)
    expect(parseAmount('4.000')).toBe(4000)
    expect(parseAmount('1.234.567')).toBe(1234567)
    expect(parseAmount('abc')).toBeNaN()
    expect(parseAmount('')).toBeNaN()
  })
})

describe('toVendorInput', () => {
  const f = { ...emptyVendorForm(), name: ' Churrasgrill ', contracted: '18.489,98', dueDate: '2027-08-23', paid: '4.000' }
  it('monta o corpo com o já pago na criação', () => {
    expect(toVendorInput(f, true)).toEqual({ input: { name: 'Churrasgrill', contracted: 18489.98, dueDate: '2027-08-23', paid: 4000 } })
  })
  it('na edição ignora o já pago', () => {
    expect(toVendorInput(f, false)).toEqual({ input: { name: 'Churrasgrill', contracted: 18489.98, dueDate: '2027-08-23' } })
  })
  it('recusa nome vazio, valor inválido e pago acima do contratado', () => {
    expect(toVendorInput({ ...f, name: ' ' }, true)).toHaveProperty('error')
    expect(toVendorInput({ ...f, contracted: '0' }, true)).toHaveProperty('error')
    expect(toVendorInput({ ...f, paid: '20000' }, true)).toHaveProperty('error')
  })
})

describe('planStatus', () => {
  it('sem fornecedores, sem prazo, quitado', () => {
    expect(planStatus(summary({ vendors: [] })).text).toMatch(/Cadastre/)
    expect(planStatus(summary({ lastDue: null })).tone).toBe('warning')
    expect(planStatus(summary({ remaining: 0 })).tone).toBe('ok')
  })
  it('atrasado é crítico e cita o fornecedor', () => {
    const s = planStatus(summary({ vendors: [vendor({ name: 'Convites', late: true })] }))
    expect(s.tone).toBe('critical')
    expect(s.text).toContain('Convites')
  })
  it('compara com a sobra', () => {
    expect(planStatus(summary({ fits: false, surplus: 500 })).tone).toBe('warning')
    expect(planStatus(summary({ fits: true, surplus: 3000 })).tone).toBe('ok')
  })
})

describe('extrato', () => {
  const tx = (over: Partial<TransactionHit>): TransactionHit => ({
    id: 't',
    date: '2026-11-01',
    description: 'PIX',
    note: '',
    amount: 100,
    kind: 'EXPENSE',
    status: 'PAID',
    ...over,
  })
  it('só saídas podem pagar fornecedor', () => {
    expect(isOutflow(tx({}))).toBe(true)
    expect(isOutflow(tx({ kind: 'TRANSFER', transferDirection: 'OUT' }))).toBe(true)
    expect(isOutflow(tx({ kind: 'INCOME' }))).toBe(false)
    expect(isOutflow(tx({ kind: 'TRANSFER', transferDirection: 'IN' }))).toBe(false)
    expect(isOutflow(tx({ status: 'CANCELLED' }))).toBe(false)
  })
  it('junta os ids já ligados', () => {
    const v = vendor({
      payments: [
        { id: 'p1', amount: 1, paidOn: '2026-01-01', transactionId: 'a', description: '' },
        { id: 'p2', amount: 1, paidOn: '2026-01-01', transactionId: null, description: '' },
      ],
    })
    expect([...linkedIds([v])]).toEqual(['a'])
  })
})
