/** Acima desta parcela das despesas do mês em "Outros", a Visão geral avisa (30%). Exatamente 30% não avisa. */
export const OTHER_SHARE_WARNING = 0.3

// Margem para o erro de ponto flutuante da divisão (30/100 não pode virar "acima de 30%").
const EPSILON = 1e-9

/** Avisa quando "Outros" passa de 30% das despesas; sem despesas no mês (null) não há o que avisar. */
export const shouldWarnOtherShare = (share: number | null | undefined): share is number =>
  share !== null && share !== undefined && share - OTHER_SHARE_WARNING > EPSILON

const percentOne = new Intl.NumberFormat('pt-BR', { style: 'percent', minimumFractionDigits: 0, maximumFractionDigits: 1 })

/** "91%" ou "30,1%": uma casa só quando precisa (para 30,1% não aparecer como "30%" num aviso de "mais de 30%"). */
export const formatShare = (share: number) => percentOne.format(share)
