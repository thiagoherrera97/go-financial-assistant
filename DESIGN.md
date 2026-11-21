---
name: FinAssist
description: Painel pessoal de finanças no padrão dos bancos digitais - o saldo é o herói, cards brancos calmos, roxo só onde há ação ou o número principal.
colors:
  accent: "#7135d4"
  accent-hover: "#5f25bd"
  accent-soft: "#efe8fb"
  accent-ink: "#5a1fb8"
  hero: "#6c2ecf"
  page: "#f4f3f8"
  surface: "#ffffff"
  surface-2: "#f0eef5"
  surface-3: "#e7e4ef"
  text-primary: "#16131d"
  text-secondary: "#4f4a5b"
  text-muted: "#6a6575"
  grid: "#e8e5ee"
  good: "#0b7a3b"
  bad: "#c0302e"
  warning: "#e5a000"
  critical: "#d03b3b"
  accent-dark: "#a98bf5"
  hero-dark: "#4c1fa3"
  page-dark: "#0e0c13"
  surface-dark: "#18151f"
  surface-2-dark: "#221e2b"
  text-primary-dark: "#f5f3f9"
  cat-food: "#eb6834"
  cat-market: "#008300"
  cat-transport: "#2a78d6"
  cat-health: "#e34948"
  cat-entertainment: "#4a3aa7"
  cat-shopping: "#e87ba4"
  cat-investment: "#1baf7a"
  cat-salary: "#eda100"
  cat-other: "#898781"
  cat-housing: "#8c5e3c"
  cat-bills: "#1597a5"
  cat-education: "#6b8e23"
  cat-people: "#b0459b"
typography:
  hero-number:
    fontFamily: "Figtree, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "3.25rem"
    fontWeight: 650
    lineHeight: 1.1
    letterSpacing: "-0.035em"
  display:
    fontFamily: "Figtree, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "1.875rem"
    fontWeight: 650
    lineHeight: 1.2
    letterSpacing: "-0.025em"
  headline:
    fontFamily: "Figtree, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "1.1875rem"
    fontWeight: 650
    lineHeight: 1.5
    letterSpacing: "-0.015em"
  title:
    fontFamily: "Figtree, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 650
    lineHeight: 1.5
    letterSpacing: "-0.01em"
  body:
    fontFamily: "Figtree, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.5
    fontFeature: "tnum"
  label:
    fontFamily: "Figtree, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 500
    lineHeight: 1.5
rounded:
  sm: "10px"
  md: "14px"
  lg: "20px"
  xl: "24px"
  pill: "999px"
spacing:
  xs: "8px"
  sm: "12px"
  md: "16px"
  lg: "20px"
  xl: "28px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "#ffffff"
    typography: "{typography.body}"
    rounded: "{rounded.pill}"
    padding: "0 16px"
    height: "40px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
  button-secondary:
    backgroundColor: "{colors.surface-2}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.pill}"
    padding: "0 16px"
    height: "40px"
  button-secondary-hover:
    backgroundColor: "{colors.surface-3}"
  card:
    backgroundColor: "{colors.surface}"
    rounded: "{rounded.lg}"
    padding: "20px"
  hero-card:
    backgroundColor: "{colors.hero}"
    textColor: "#ffffff"
    rounded: "{rounded.xl}"
    padding: "28px"
  input:
    backgroundColor: "{colors.surface-2}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.sm}"
    padding: "8px 12px"
    height: "40px"
  nav-link:
    textColor: "{colors.text-secondary}"
    rounded: "12px"
    padding: "8px 12px"
    height: "40px"
  nav-link-active:
    backgroundColor: "{colors.accent-soft}"
    textColor: "{colors.accent-ink}"
  chip-category:
    rounded: "12px"
    size: "40px"
---

# Design System: FinAssist

## Overview

**Creative North Star: "O Banco Pessoal"**

O FinAssist se veste como um banco digital de uma pessoa só: o saldo é o herói, cada tela é uma pilha calma de cards brancos sobre um fundo levemente violeta, e o roxo aparece apenas onde há ação (botão, item ativo do menu) ou onde mora o número principal (o card-herói). Nubank é referência de nível de acabamento e clareza, nunca de marca: sem logo, nome ou roxo exato dele.

A densidade é de uso diário: olhada rápida no celular, revisão longa no desktop, com a mesma linguagem de componentes nos dois. Cor informa (categoria, bom/ruim, alerta), nunca decora. Valores são grandes, em peso 650 com tracking negativo e algarismos tabulares. O sistema recusa o painel cinza genérico de admin, com borda fina em tudo.

**Key Characteristics:**
- Fundo neutro com leve tinta violeta; cards brancos sem borda e com sombra suave (borda só no tema escuro).
- Raio generoso (20px em cards, 24px no herói), botões em pílula.
- Figtree em todo texto; números tabulares sempre.
- Roxo como único acento de interface; paleta de dataviz fixa para categorias e séries.
- Tema claro e escuro (preferência do sistema, com o botão de tema vencendo via `data-theme`).

## Colors

Neutros violeta-tingidos, um roxo único de ação e uma paleta de dataviz validada para categorias. Os valores abaixo seguem o tema claro; o escuro tem contrapartes no código (`*-dark` no frontmatter para os principais).

### Primary
- **Roxo FinAssist** (#7135d4, `--accent`): botões primários, foco, seleção, séries principais. No escuro vira Lavanda Noturna (#a98bf5) com texto #150a2c.
- **Roxo Profundo de Hover** (#5f25bd, `--accent-hover`): estado hover do botão primário.
- **Tinta Roxa** (#5a1fb8, `--accent-ink`): texto e ícone roxos sobre fundo claro (links, item ativo do menu).
- **Véu Lilás** (#efe8fb, `--accent-soft`): fundo do item ativo e do botão pressionado. Avisos usam um tom de âmbar (`--warning` a 13% sobre a superfície), para o lilás não competir com a ação.
- **Roxo do Herói** (#6c2ecf, `--hero`; escuro #4c1fa3): faixa do card-herói e barra superior do celular, sempre com texto branco.

### Neutral
- **Página Violeta-Névoa** (#f4f3f8, `--page`; escuro #0e0c13): fundo da aplicação.
- **Card Branco** (#ffffff, `--surface`; escuro #18151f): cards, menu lateral.
- **Tom de Campo** (#f0eef5, `--surface-2`; escuro #221e2b): inputs, botões secundários, blocos tonais dentro de cards, trilhas de barra.
- **Tom Profundo** (#e7e4ef, `--surface-3`; escuro #2c2737): hover de secundário, controle segmentado.
- **Tinta Violeta** (#16131d, `--text-primary`), **Texto Secundário** (#4f4a5b), **Texto Mudo** (#6a6575): hierarquia de texto, todos AA sobre as superfícies.
- **Linha de Grade** (#e8e5ee, `--grid`): divisórias de tabela e de corpo de card.

### Semantic
- **Bom** (#0b7a3b; escuro #2fbf62), **Ruim** (#c0302e; escuro #f07272), **Alerta** (#e5a000), **Crítico** (#d03b3b): entradas, saídas, medidores, selos essencial/supérfluo.

### Categorias (dataviz)
Cores fixas por categoria, aplicadas via `--cat` em ícones-chip (fundo 15% da própria cor), barras de grupo e barras de participação: alimentação #eb6834, mercado #008300, transporte #2a78d6, saúde #e34948, lazer #4a3aa7, compras #e87ba4, investimento #1baf7a, salário #eda100, outros #898781, moradia #8c5e3c, contas #1597a5, educação #6b8e23, pessoas #b0459b. Séries: azul, laranja, verde, âmbar (`--series-1..4`); o roxo não entra nos gráficos, fica reservado para ação e para o herói.

### Named Rules
**The Purple Means Act Rule.** Roxo aparece só em ação (botão, foco, item ativo), no número principal (herói) e na série 1 de gráficos. Se uma superfície não tem ação nem número principal, ela não tem roxo.
**The Category Colors Are Fixed Rule.** Uma categoria tem sempre a mesma cor em todas as telas e nos dois temas (com a variante escura já definida); nunca reatribuir por estética.
**The Tinted Neutral Rule.** Neutros carregam um fio de violeta (matiz ~280). Nunca cinza puro, nunca #000 ou #fff como texto/fundo de página (o branco puro é reservado ao card).

## Typography

**Display Font:** Figtree (fallback system-ui, -apple-system, Segoe UI, sans-serif), self-hosted em `/fonts/figtree-latin.woff2` e `figtree-latin-ext.woff2`, eixo variável 100-900.
**Body Font:** Figtree.
**Label/Mono Font:** Figtree; `--font-mono` (ui-monospace) existe mas não é papel de interface.

**Character:** Uma só família humanista e amigável, com a hierarquia vinda de peso (650) e tracking negativo nos números e títulos, não de troca de fonte. `font-variant-numeric: tabular-nums` é global no `body`.

### Hierarchy
- **Hero number** (650, clamp(2.25rem, 6vw, 3.25rem), 1.1, -0.035em): total em conta no card-herói.
- **Display / page-title** (650, 1.875rem, 1.2, -0.025em; 1.625rem no celular): título de cada tela.
- **Headline / section-title** (650, 1.1875rem, -0.015em): seções dentro da tela.
- **Title** (650, 1.0625rem, -0.01em): títulos de card e gráfico.
- **Tile value** (650, 1.625rem, -0.025em) e saldo de banco (650, 1.875rem, -0.03em): números de destaque em cards.
- **Body** (400, 1rem/1.5; 0.875-0.9375rem em controles e tabelas).
- **Label** (500, 0.8125rem; 0.75rem para metadados e rótulos de menu): rótulos de tile, cabeçalhos de tabela, notas.

### Named Rules
**The Number First Rule.** O valor monetário é sempre o maior elemento do seu card, em 650 com tracking negativo; o rótulo fica menor, em peso 500 e cor secundária.
**The One Family Rule.** Figtree em tudo. Hierarquia por tamanho, peso e cor, não por segunda fonte.

## Layout

Aplicação em duas colunas a partir de 900px: menu lateral fixo de 256px e conteúdo em `.container` (máx. 1160px, padding lateral de 40px no desktop e 20px no celular). Abaixo de 900px o menu vira gaveta com scrim e aparece uma barra superior roxa sticky (menu, marca, sincronizar). O ritmo vertical é de pilha: cards separados por 12-16px, seções por 32-36px, título de página com 20px abaixo.

Grades usam `repeat(auto-fit/auto-fill, minmax(...))`: tiles de 190px, cards de banco de 300px, grupos de 320px, contas de 280px, com gap de 12-16px. O Painel abre com o card-herói em largura total, seguido de cards dos bancos em grade. Em telas até 720px, cards caem para padding 16px e raio 14px, colunas secundárias de tabela somem e seus dados descem para baixo da descrição. Alvos de toque mínimos de 40px (36px em ícones, 32px nos botões pequenos).

Escala de espaçamento observada: 8, 12, 16, 20, 28.

## Elevation & Depth

Híbrido: no claro, profundidade vem de sombra suave e difusa tingida de violeta sobre cards brancos sem borda; no escuro, a sombra some e o card se separa por uma borda de 1px a 6% de branco mais a diferença tonal de superfície.

### Shadow Vocabulary
- **Repouso** (`box-shadow: 0 1px 2px rgba(35,20,70,.05), 0 2px 8px rgba(35,20,70,.04)`, `--shadow-sm`; escuro: nenhuma): cards, cards de grupo, contas.
- **Elevado** (`box-shadow: 0 2px 4px rgba(35,20,70,.05), 0 12px 32px rgba(35,20,70,.1)`, `--shadow-md`; escuro: `0 12px 32px rgba(0,0,0,.45)`): hover de tile-link e grupo, grupo aberto, tooltip de gráfico.
- **Gaveta** (`8px 0 40px rgba(20,10,40,.25)`): só no menu aberto no celular.

### Named Rules
**The Card Inside Card Rule.** Card dentro de card vira bloco tonal (`--surface-2`), sem sombra e sem borda; nunca duas sombras empilhadas.
**The Shadow Is Soft Rule.** Sombras são difusas e deslocadas para baixo, tingidas de violeta. Nenhuma sombra dura com offset sem desfoque.

## Shapes

Raios generosos e consistentes: 24px (herói), 20px (cards), 14px (avisos, formulários internos, cards no celular), 10px (campos de formulário), 12px (ícone-chip de categoria e item de menu), 999px (botões, selos, status, barras, controle segmentado). Cartões de banco usam 16px com proporção 85.6/54 e gradiente da cor do banco. Marcas de banco são círculos. Barras de progresso e medidores têm 4-10px de altura e ponta totalmente arredondada. Sem faixas laterais coloridas: avisos usam fundo tonal.

## Components

### Buttons
- **Shape:** pílula (999px), 40px de altura, padding 0 16px, peso 500, texto 0.9375rem.
- **Primary:** fundo roxo (#7135d4), texto branco, peso 600; hover escurece para #5f25bd; `:active` escala 0.98.
- **Secondary (padrão `.btn`):** fundo `--surface-2`, texto primário; hover `--surface-3`.
- **Pressed (`aria-pressed`):** fundo Véu Lilás, texto Tinta Roxa, peso 600.
- **Small / Icon:** 32px de altura com 0.8125rem; ícone 36px redondo. No celular, botões sobre a barra roxa ficam em branco translúcido (16%).

### Cards / Containers
- **Corner Style:** 20px (14px no celular).
- **Background:** branco; interno vira `--surface-2`.
- **Shadow Strategy:** ver Elevação; repouso suave, elevado no hover de cards clicáveis (com `translateY(-1px)`).
- **Border:** transparente no claro, 1px branco a 6% no escuro.
- **Internal Padding:** 20px (16px no celular; grupos usam 16-18px).

### Card-herói (assinatura)
Card de largura total com faixa roxa (`--hero`) no topo, raio 24px, rótulo em branco 78%, número em 3.25rem branco, faixa lisa sem enfeite; abaixo, corpo branco com barra de participação por banco (10px, pílula, segmentos com 3px de vão; só aparece com dois ou mais bancos com saldo) e legenda. Aparece uma vez por tela, no Painel.

### Inputs / Fields
- **Style:** 40px, borda 1px a 8%, fundo `--surface-2`, raio 10px; selects com chevron próprio.
- **Focus:** borda roxa, fundo sobe para branco e anel de 3px roxo a 22%. Hover reforça a borda. Desabilitado a 55%.

### Navigation
Menu lateral branco de 256px: marca (quadrado roxo 30px com inicial), botão sincronizar, grupos com título 0.75rem em cinza mudo, itens de 40px, raio 12px, ícone + texto 0.9375rem. Ativo: fundo Véu Lilás, texto e ícone em Tinta Roxa, peso 600. Rodapé com borda superior. No celular: barra roxa sticky e gaveta com scrim.

### Chips de categoria, selos e status
Ícone-chip: quadrado arredondado (12px, 40px) com ícone na cor da categoria sobre 15% da mesma cor. Selos e status: pílula 0.6875-0.75rem sobre `--surface-2`; variantes bom/ruim usam a cor semântica a 11-12%.

### Controle segmentado e tabelas
Segmentado: trilha pílula `--surface-2`, opção ativa em superfície branca com sombra suave. Tabelas de dados: sem zebra, divisórias de 1px `--grid`, números à direita, cabeçalho 0.8125rem mudo, hover de linha tonal.

## Do's and Don'ts

### Do:
- **Do** deixar o número principal de cada card como o maior elemento, em 650 com tracking negativo e `tabular-nums`.
- **Do** usar roxo só em ação, foco, item ativo, herói e série 1.
- **Do** usar variáveis (`--surface`, `--accent`, `--shadow-sm`, `--r-lg`...) e nunca hex solto, para que o tema escuro funcione.
- **Do** reaproveitar `.card`, `.btn`, `.chip`, `.badge`, `.status` antes de criar variante nova.
- **Do** manter foco visível (2px roxo, offset 2px) e respeitar `prefers-reduced-motion`.
- **Do** garantir contraste AA nos dois temas; texto sobre o herói é sempre branco.

### Don't:
- **Don't** voltar ao painel cinza de admin: bordas finas em todo card, fundo neutro puro, sem hierarquia de número.
- **Don't** usar o logo, o nome ou o roxo exato do Nubank; ele é referência de acabamento, não de marca.
- **Don't** empilhar sombra em card dentro de card, nem usar sombra dura sem desfoque.
- **Don't** usar faixa lateral colorida em avisos; use fundo tonal.
- **Don't** colorir por decoração: cor só informa categoria, bom/ruim ou alerta.
- **Don't** introduzir uma segunda família tipográfica nem números proporcionais em valores.
- **Don't** reatribuir a cor de uma categoria entre telas.
