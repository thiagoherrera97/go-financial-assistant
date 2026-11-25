/**
 * A Capi, mascote do FinAssist: uma capivara que reage aos números.
 * happy: sobrou dinheiro (olhos fechados de alegria, a tangerina na cabeça e brilhos).
 * neutral: tudo normal (pisca de vez em quando).
 * worried: algo aperta (sobrancelhas, boca tremida e uma gota de suor).
 * Desenho próprio em SVG, cores fixas nos dois temas (é um personagem, não um ícone de interface).
 */
export type CapiMood = 'happy' | 'neutral' | 'worried'

const INK = '#3a2617'

export function Capi({ mood = 'neutral', size = 96, className = '' }: { mood?: CapiMood; size?: number; className?: string }) {
  return (
    <svg
      className={`capi capi-${mood} ${className}`}
      width={size}
      height={size}
      viewBox="0 0 120 120"
      aria-hidden="true"
      focusable="false"
    >
      <g className="capi-body">
        {/* corpo e patinhas */}
        <ellipse cx="60" cy="90" rx="34" ry="24" fill="#a06f48" />
        <ellipse cx="44" cy="111" rx="9" ry="5.5" fill="#7d5233" />
        <ellipse cx="76" cy="111" rx="9" ry="5.5" fill="#7d5233" />
        <ellipse cx="60" cy="96" rx="18" ry="13" fill="#c39670" opacity="0.55" />
        {/* orelhas */}
        <circle cx="37" cy="33" r="8" fill="#7d5233" />
        <circle cx="83" cy="33" r="8" fill="#7d5233" />
        <circle cx="37" cy="34" r="3.8" fill="#5c3a22" />
        <circle cx="83" cy="34" r="3.8" fill="#5c3a22" />
        {/* cabeça quadradona, como capivara de verdade */}
        <rect x="24" y="30" width="72" height="60" rx="27" fill="#b9855a" />
        <rect x="30" y="34" width="60" height="22" rx="11" fill="#c79367" opacity="0.6" />
        {/* focinho */}
        <rect x="38" y="58" width="44" height="28" rx="14" fill="#8a5b39" />
        <ellipse cx="52" cy="66" rx="3" ry="2.2" fill={INK} />
        <ellipse cx="68" cy="66" rx="3" ry="2.2" fill={INK} />
        {/* bochechas */}
        <circle cx="32" cy="66" r="5.5" fill="#f19c86" opacity="0.6" />
        <circle cx="88" cy="66" r="5.5" fill="#f19c86" opacity="0.6" />
      </g>

      {mood === 'happy' && (
        <>
          <path d="M40 50 q6 -7 12 0" stroke={INK} strokeWidth="3.2" fill="none" strokeLinecap="round" />
          <path d="M68 50 q6 -7 12 0" stroke={INK} strokeWidth="3.2" fill="none" strokeLinecap="round" />
          <path d="M51 75 q9 8 18 0" stroke={INK} strokeWidth="2.8" fill="#5c2b1d" strokeLinecap="round" strokeLinejoin="round" />
          <g className="capi-orange">
            <circle cx="60" cy="22" r="9.5" fill="#ff9a1f" />
            <circle cx="57" cy="19" r="2.6" fill="#ffc46b" />
            <path d="M60 13 q5 -6 10 -3 q-4 5 -10 3z" fill="#3fae5a" />
          </g>
          <path className="capi-spark" d="M14 34 l3 7 l7 3 l-7 3 l-3 7 l-3 -7 l-7 -3 l7 -3z" fill="#ffd166" />
          <path className="capi-spark capi-spark-2" d="M104 46 l2 5 l5 2 l-5 2 l-2 5 l-2 -5 l-5 -2 l5 -2z" fill="#ffd166" />
        </>
      )}

      {mood === 'neutral' && (
        <>
          <g className="capi-eyes">
            <circle cx="46" cy="50" r="4" fill={INK} />
            <circle cx="74" cy="50" r="4" fill={INK} />
            <circle cx="47.4" cy="48.6" r="1.3" fill="#fff" />
            <circle cx="75.4" cy="48.6" r="1.3" fill="#fff" />
          </g>
          <path d="M54 77 q6 4 12 0" stroke={INK} strokeWidth="2.6" fill="none" strokeLinecap="round" />
        </>
      )}

      {mood === 'worried' && (
        <>
          <circle cx="46" cy="52" r="4" fill={INK} />
          <circle cx="74" cy="52" r="4" fill={INK} />
          <path d="M38 44 l13 -5" stroke={INK} strokeWidth="2.8" strokeLinecap="round" />
          <path d="M82 44 l-13 -5" stroke={INK} strokeWidth="2.8" strokeLinecap="round" />
          <path d="M52 79 q4 -4 8 0 q4 4 8 0" stroke={INK} strokeWidth="2.6" fill="none" strokeLinecap="round" />
          <path className="capi-sweat" d="M96 36 q6 9 0 13 q-6 -4 0 -13z" fill="#8fd3ff" />
        </>
      )}
    </svg>
  )
}
