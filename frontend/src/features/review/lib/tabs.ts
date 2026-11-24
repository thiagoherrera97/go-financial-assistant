/** Abas da Revisão, guardadas na URL (?aba=). */
export type ReviewTab = 'cortar' | 'gastos' | 'categorias'

const TABS: ReviewTab[] = ['categorias', 'cortar', 'gastos']

/** Aba ausente ou desconhecida abre na primeira, a das categorias. */
export const parseTab = (raw: string | null): ReviewTab => (TABS.includes(raw as ReviewTab) ? (raw as ReviewTab) : 'categorias')
