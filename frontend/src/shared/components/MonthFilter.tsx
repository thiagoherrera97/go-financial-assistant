import { shiftMonth } from "../lib/months";

interface Props {
  month: string;
  now: string;
  onChange: (month: string) => void;
  /** Avisa, no mês atual, que os valores são parciais (telas que comparam com o mês anterior ou projetam). */
  warnPartial?: boolean;
}

/** Filtro de período (o ciclo do salário, pelo mês em que começa): uma linha acima de tudo o que ele controla. */
export function MonthFilter({ month, now, onChange, warnPartial }: Props) {
  return (
    <>
      <div className="filters">
        <button
          type="button"
          className="btn"
          aria-label="Ciclo anterior"
          onClick={() => onChange(shiftMonth(month, -1))}
        >
          ←
        </button>
        <input
          type="month"
          aria-label="Ciclo (mês em que começa)"
          value={month}
          max={now}
          onChange={(e) => onChange(e.target.value)}
        />
        <button
          type="button"
          className="btn"
          aria-label="Próximo ciclo"
          disabled={month >= now}
          onClick={() => onChange(shiftMonth(month, 1))}
        >
          →
        </button>
      </div>
      {warnPartial && month === now && (
        <p className="notice partial-notice" role="note">
          Ciclo em andamento: os valores são parciais, só até hoje. Comparar com
          um ciclo completo exagera as quedas; os números só valem de verdade
          quando o ciclo fechar.
        </p>
      )}
    </>
  );
}
