// Um mês é sempre "AAAA-MM", o mesmo formato da API.

const MONTH_RE = /^(\d{4})-(0[1-9]|1[0-2])$/

export function isMonth(value: string | null | undefined): value is string {
  return value != null && MONTH_RE.test(value)
}

// O "mês atual" é o ciclo do salário de hoje, que a API informa (/api/me): perto do fim do mês do calendário ainda pode ser
// o ciclo do mês anterior. Até a API responder (ou nos testes), vale o mês do calendário.
let cycleMonth: string | null = null

export function setCurrentCycle(month: string | null) {
  cycleMonth = month != null && isMonth(month) ? month : null
}

export function currentMonth(date?: Date): string {
  if (date === undefined && cycleMonth) return cycleMonth
  const now = date ?? new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

export function shiftMonth(month: string, delta: number): string {
  const [year, mon] = month.split('-').map(Number)
  const index = year * 12 + (mon - 1) + delta
  return `${Math.floor(index / 12)}-${String((index % 12) + 1).padStart(2, '0')}`
}

/** Até quantos meses à frente há parcelas de cartão previstas (backend: domain.CardForecastMonthsAhead). */
export const FORECAST_MONTHS_AHEAD = 3
