/** Abas da Revisão, guardadas na URL (?aba=). */
export type ReviewTab = 'cortar' | 'gastos' | 'categorias'

const TABS: ReviewTab[] = ['cortar', 'gastos', 'categorias']

/** Aba ausente ou desconhecida abre em "o que dá para cortar". */
export const parseTab = (raw: string | null): ReviewTab => (TABS.includes(raw as ReviewTab) ? (raw as ReviewTab) : 'cortar')
