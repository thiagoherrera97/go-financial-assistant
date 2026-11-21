# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Uma pessoa (o dono da instalação) acompanhando as próprias finanças. Abre o dashboard tanto no desktop quanto no celular, com a mesma frequência: no celular para uma olhada rápida em saldos, fatura e gastos do dia; no desktop para revisar o mês, classificar transações, ajustar orçamento e metas.

## Product Purpose

FinAssist junta bancos (via Open Finance / Meu Pluggy), um bot próprio no Telegram e um dashboard no navegador. O dashboard mostra saldos, faturas de cartão, gastos por categoria, orçamento, metas, revisão de gastos com sugestões de corte, projeção e investimentos. Sucesso: em segundos a pessoa sabe quanto tem, quanto deve e onde o dinheiro foi, e consegue corrigir uma categoria ou anotar uma transação sem atrito.

## Positioning

Auto-hospedado, dados no banco de dados da própria pessoa, transações chegando sozinhas pelo Open Finance e conversa pelo Telegram. Não é um app de banco nem um SaaS: é o painel pessoal de uma instalação única.

## Operating Context

Uso diário e rápido no celular; sessões de revisão mais longas no desktop. Valores em reais (BRL), datas e horários de São Paulo, interface toda em português do Brasil. Existe opção de ocultar valores (uso em público).

## Capabilities and Constraints

- Frontend React 19 + Vite + TypeScript, React Router, TanStack Query, Recharts, lucide-react. CSS próprio (sem framework de UI).
- Telas: Painel, Mês, Visão geral, Transações, Gastos, Orçamento, Revisão, Metas, Fornecedores, Coach (IA), Projeção, Investimentos, Comparações, Classificar, Configurações, Login, Setup.
- Tema claro e escuro (preferência do sistema + botão que vence).
- Paleta de dataviz validada para categorias e séries (cores por categoria fixas).
- Login por senha com segundo fator no chat.

## Brand Commitments

Nome: FinAssist. Tom pedido pelo dono: calmo e preciso — limpo, números em destaque, cor só onde informa.

Preferência permanente (escolhida no redesign de nov/2026): seguir o padrão dos bancos digitais, com o Nubank como referência de acabamento e clareza. É referência de nível, não de marca: sem logo, nome ou roxo exato do Nubank.

## Evidence on Hand

Dados reais do usuário vindos do backend local; nenhuma afirmação comercial, depoimento ou métrica externa existe e nenhuma deve ser inventada.

## Product Principles

1. O número vem primeiro: saldo, fatura e gasto legíveis de relance em qualquer tela.
2. Cor informa, não decora: categoria, bom/ruim, alerta.
3. Mesma linguagem de componentes em todas as telas.
4. Celular e desktop são cidadãos iguais.
5. Corrigir é barato: classificar, anotar e ajustar sem sair do contexto.

## Accessibility & Inclusion

Contraste WCAG AA nos dois temas, foco visível por teclado, respeito a prefers-reduced-motion, números tabulares.
