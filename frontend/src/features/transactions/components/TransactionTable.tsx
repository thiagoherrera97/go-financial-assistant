import { describeAmount, formatBankTime, formatDay } from '../lib/transactions'
import { CategoryChip } from '../../../shared/components/CategoryChip'
import { DeleteManual } from './DeleteManual'
import { NoteEditor } from './NoteEditor'
import { ClassificationEditor } from './ClassificationEditor'
import { EssentialBadge } from './EssentialBadge'
import type { Transaction } from '../api'

const categoryText = (t: Transaction) => (t.subcategoryLabel ? `${t.categoryLabel} › ${t.subcategoryLabel}` : t.categoryLabel)

/** Tabela de lançamentos: data, descrição e valor primeiro. Em cards estreitas categoria e conta descem para baixo da descrição. */
export function TransactionTable({ items, hideCategory = false }: { items: Transaction[]; hideCategory?: boolean }) {
  return (
    <div className="table-scroll">
      <table className="data tx">
        <thead>
          <tr>
            <th scope="col">Data</th>
            <th scope="col">Descrição</th>
            <th scope="col" className="amount-col">
              Valor
            </th>
            {!hideCategory && (
              <th scope="col" className="col-wide">
                Categoria
              </th>
            )}
            <th scope="col" className="col-wide">
              Pagamento
            </th>
            <th scope="col" className="col-wide">
              Conta
            </th>
          </tr>
        </thead>
        <tbody>
          {items.map((t) => {
            const a = describeAmount(t)
            const time = formatBankTime(t.bankTime)
            return (
              <tr key={t.id}>
                <td>
                  {formatDay(t.date)}
                  {time && (
                    <span className="amount-caption" title="Hora informada pelo banco">
                      {time}
                    </span>
                  )}
                </td>
                <td className="left">
                  {t.description || '—'}
                  {t.status === 'PENDING' && <span className="badge">Pendente</span>}
                  {t.source === 'MANUAL' && <span className="badge">Manual</span>}
                  {hideCategory && t.kind === 'EXPENSE' && (
                    <span className="tx-meta always">
                      {t.subcategoryLabel}
                      <EssentialBadge essential={t.essential} />
                    </span>
                  )}
                  <span className="tx-meta">
                    {[hideCategory ? '' : categoryText(t), t.accountName].filter(Boolean).join(' · ')}
                    {!hideCategory && t.kind === 'EXPENSE' && <EssentialBadge essential={t.essential} />}
                  </span>
                  <span className="tx-actions">
                    <NoteEditor key={`${t.id}:${t.note}`} id={t.id} note={t.note} />
                    <ClassificationEditor key={`${t.id}:${t.category}:${t.subcategory}`} tx={t} />
                    {t.source === 'MANUAL' && t.type === 'SINGLE' && <DeleteManual id={t.id} />}
                  </span>
                </td>
                <td className={`amount amount-${a.tone}`}>
                  {a.text}
                  <span className="amount-caption">{a.caption}</span>
                </td>
                {!hideCategory && (
                  <td className="left col-wide">
                    <span className="cat-cell">
                      <CategoryChip category={t.category} size={14} />
                      {categoryText(t)}
                    </span>
                    {t.kind === 'EXPENSE' && <EssentialBadge essential={t.essential} />}
                  </td>
                )}
                <td className="left col-wide">{t.paymentMethodLabel}</td>
                <td className="left col-wide">{t.accountName || '—'}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
