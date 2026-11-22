export type Tone = 'ok' | 'warning' | 'critical'

/** Cor de um valor de dinheiro: verde (entra, sobra), vermelho (sai, falta). */
export type SignTone = 'good' | 'bad'

/** Verde se positivo, vermelho se negativo, sem cor se zerado (centavos de arredondamento contam como zero). */
export const signTone = (value: number): SignTone | undefined => (value >= 0.005 ? 'good' : value <= -0.005 ? 'bad' : undefined)
