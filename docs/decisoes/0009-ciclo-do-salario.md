# 0009 — O "mês" é o ciclo do salário

**Contexto.** O salário cai em duas partes (dia 15 e dia 30) e paga o financiamento (dia 30) e a fatura do cartão (dia 10 do mês seguinte). Com as telas cortando no dia 1, a fatura aparecia num mês diferente do salário que a paga, e a "sobra do mês" não correspondia a dinheiro real.

**Decisão.** Todas as telas agrupam pelo ciclo do salário: da 1ª parte do salário (no dia útil, ou antes, se o crédito chegou adiantado) até a véspera da próxima. A chave continua `AAAA-MM` (o mês em que o ciclo começa), então orçamento, metas e a API não mudam de forma. Os ciclos ficam numa tabela gerada pelo Go (feriados e dias úteis só existem no Go) e o SQL usa `cycle_of`. Sem salário configurado, é o mês do calendário.

**Consequências.** Mudar o dia do salário recalcula os ciclos passados. Um salário adiantado faz os gastos daquele dia mudarem de ciclo depois da sincronização. Dívidas e fornecedores continuam em datas reais de vencimento.

**Atualização (2026-11-23).** Para outra pessoa usar o app, a regra virou configuração (`CYCLE_START`): salário (padrão), dia fixo (`CYCLE_DAY`) ou calendário. A chave `AAAA-MM` e a tabela `cycles` não mudam.
