---
version: 1
slug: "frontend-src"
primary_target: "frontend/src"
related_targets: []
---

# Dashboard do FinAssist (todas as telas autenticadas + login/setup)

Modo: Operate. Público: o dono da instalação, no celular (olhada rápida) e no desktop (revisão do mês).

## Direction contract

THESIS: O FinAssist como um banco digital pessoal: o saldo é o herói, cada tela é uma pilha calma de cards brancos e o roxo só aparece onde há ação ou o número principal. Recusa o painel cinza genérico de admin com bordas finas em tudo.

OWN-WORLD: Fundo neutro levemente violeta (#f5f4f8 claro, #0f0d14 escuro), cards brancos sem borda com sombra suave deslocada e raio 20px, roxo FinAssist (#7135d4 / #a98bf5 no escuro) em botões pílula, item ativo do menu e no card-herói preenchido. Figtree em todos os textos, números tabulares, valores grandes em 600 com tracking negativo. Cores de categoria da paleta de dataviz mantidas.

STORY: A pessoa abre e vê quanto tem (herói roxo), quanto deve (faturas) e onde gastou; corrige categoria ou anota sem sair da tela.

FIRST VIEWPORT: Desktop: menu lateral branco (240px) com marca, sincronizar e grupos; conteúdo à direita com título grande e, no Painel, o card-herói roxo de largura total com o total em conta (≈3.25rem branco), barra de participação e fatura à parte; abaixo, cards dos bancos em grade. Celular: barra superior roxa com menu, marca e sincronizar; herói logo abaixo. Ação principal: "Ocultar valores"/"Sincronizar" como botões pílula.

FORM: canon (o padrão da categoria, referência de acabamento: Nubank), escolhido pelo usuário após a rolagem; seed:261c14e3938108a6.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
