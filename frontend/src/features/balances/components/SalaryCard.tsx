import { CircleCheck, Clock } from 'lucide-react'
import { formatMonthLong } from '../../../shared/lib/format'
import { salaryPartText } from '../lib/salary'
import { StaticMoney } from './Money'
import type { Salary } from '../api'

/** O salário do mês: cada parte, quando cai (dia útil anterior se preciso) e se já caiu; o que falta receber. */
export function SalaryCard({ salary, hidden }: { salary: Salary; hidden: boolean }) {
  if (!salary.configured) {
    return <p className="bal-aside">Configure as partes do salário em Configurações → Renda para ver o que falta receber no mês.</p>
  }
  return (
    <section className="bal-salary" aria-labelledby="bal-salary-title">
      <div className="bal-salary-head">
        <h2 id="bal-salary-title">Salário de {formatMonthLong(salary.month)}</h2>
        <p>
          {salary.pending > 0 ? (
            <>
              A receber <strong><StaticMoney value={salary.pending} hidden={hidden} /></strong>
            </>
          ) : (
            'Tudo recebido'
          )}
        </p>
      </div>
      <ul>
        {salary.parts.map((p) => (
          <li key={p.day} className={p.received ? 'is-received' : undefined}>
            {p.received ? <CircleCheck size={16} aria-label="recebido" /> : <Clock size={16} aria-label="a receber" />}
            <span className="bal-salary-day">Dia {p.day}</span>
            <span className="bal-salary-when">{salaryPartText(p)}</span>
            <span className="bal-salary-amount">
              <StaticMoney value={p.received && p.receivedAmount !== null ? p.receivedAmount : p.amount} hidden={hidden} />
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}
