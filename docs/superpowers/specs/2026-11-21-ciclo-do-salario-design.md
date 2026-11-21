# Ciclo do salário no lugar do mês do calendário

Data: 2026-11-21 · Status: rascunho para aprovação

## Por quê

O dinheiro do Thiago não vive do dia 1 ao 31. O salário cai em duas partes (hoje dia 15 e dia 30), e é com ele que
se paga o financiamento (dia 30) e a fatura do cartão (dia 10 do mês seguinte). O que sobra depois disso, até o
próximo salário, é a sobra de verdade, que se soma ao salário seguinte. Ver isso cortado no dia 1 faz ele "se perder":
a fatura paga com o salário de novembro aparece em dezembro, e a sobra do mês não corresponde a nada real.

## O que é o ciclo

- **Ciclo de novembro** = do dia em que cai a 1ª parte do salário em novembro até a véspera da 1ª parte de dezembro.
  Com o salário atual: **15/nov → 14/dez**.
- O nome do ciclo é o mês em que ele começa. A API continua recebendo `AAAA-MM`; passa a significar "o ciclo que
  começa nesse mês".
- **Início do ciclo:** o dia da 1ª parte de `SALARY_SCHEDULE`, ajustado para o dia útil anterior (mesma regra de
  `domain.SalaryPayDate`). Se o crédito real dessa parte (categoria Salário, casado pela regra de `SalaryMonth`) cair
  antes, o ciclo começa no dia do crédito real. O ciclo começa quando o dinheiro entra. (Confirmado pelo Thiago: os
  gastos desses dias mudam de ciclo depois da sincronização que traz o salário, e isso é aceito.) O casamento crédito ↔
  parte usa a data combinada do calendário, sem depender do ciclo, e só a 1ª parte define o início.
- **Sem salário configurado:** o ciclo é o mês do calendário (início no dia 1). Nada quebra para quem não configurou.
- Se o dia do salário mudar, os ciclos passam a seguir o novo dia (inclusive os passados, que são recalculados).
  Decisão consciente: o app é pessoal e o histórico refeito com a regra nova é aceitável.

## Em que ciclo cada lançamento cai

A regra de hoje fica igual, só que trocando "mês" por "ciclo":

- **Compra no cartão:** pelo **vencimento da fatura** (`bill_due_date`). A fatura que vence 10/dez cai no ciclo de
  novembro, que é o salário que a paga. A atribuição compra → fatura já existe e segue o **fechamento** (do banco ou
  estimado por `CARD_CLOSE_DAYS_BEFORE_DUE`). Por isso uma compra feita depois do fechamento vai para a fatura seguinte
  e, com ela, para o ciclo seguinte.
- **Todo o resto (Pix, débito, boleto, financiamento, salário):** pela data da transação, como hoje.
- Não existe mais a ideia de "cartão principal": cada cartão segue o vencimento dele. Trocar de cartão ou mudar a data
  de vencimento não pede nenhuma configuração.

## Escopo: o app inteiro (escolhido)

Todas as telas e consultas que hoje agrupam por mês passam a agrupar por ciclo: Mês (vira **Ciclo**), Visão geral,
Orçamento, Gastos/subcategorias, Revisão e economia realizada, Metas, Fornecedores, Classificar, Transações, Dívidas,
Coach, Projeção, investimentos ao fim do período, resumo do chat e exportação CSV.

A abordagem combinada é ir com tudo e ajustar o que aparecer errado no uso.

### Novidades da tela Ciclo

- O título vira "Ciclo de novembro", com o período abaixo: "15/nov – 14/dez".
- Linha **"Veio do ciclo anterior"**: a sobra do ciclo anterior (entradas − despesas daquele ciclo). Não é acumulada
  desde o começo. A "sobra" atual ganha a versão "contando o que veio do anterior".
- O aviso de "em andamento" vale enquanto hoje for antes do fim do ciclo.

## Abordagens consideradas

1. **Só a tela Mês vira ciclo.** Rápido, mas os números deixam de bater com Orçamento e Visão geral. Descartada pelo Thiago.
2. **O app inteiro por ciclo (escolhida).** Coerente em todas as telas. É uma mudança grande, mas concentrada:
   quase tudo passa por `paymentMonth`/`txDate` em `infra/db/queries_common.go` (41 usos em 13 arquivos).
3. **Opção nas configurações (calendário ou ciclo).** Tem o custo da 2 e ainda obriga a manter os dois jeitos para
   sempre. Descartada.

## Como (decisões técnicas, minhas)

- **Tabela `cycles`** (`month date PK, starts_on date`), gerada em Go (feriados e dia útil já estão em
  `domain/business_day.go`, que o SQL não tem) para uma janela larga (desde o lançamento mais antigo até 2 anos à
  frente). É regravada no boot, quando `SALARY_SCHEDULE` muda e depois de cada sincronização (por causa do crédito real
  adiantado).
- **`paymentMonth` vira `paymentCycle`**: o `month` da maior linha de `cycles` com `starts_on <= data efetiva` (a mesma
  data efetiva de hoje: `bill_due_date`, depois `due_date`, etc.). Todas as consultas continuam devolvendo uma
  "chave de mês" `AAAA-MM-01`, então os contratos da API e os gráficos não mudam de forma.
- **Do lado do Go**, tudo que hoje calcula "mês atual", "primeiro/último dia do mês" ou "mês em andamento" passa a
  perguntar a um único ponto (`domain.Cycle`/um serviço de ciclos): qual o ciclo de uma data, e início/fim de um ciclo.
  Inclui `SalaryMonth` (as partes do salário de um ciclo são as que caem dentro dele), investimentos (saldo no fim do
  ciclo), projeção e chat.
- O frontend troca "mês" por "ciclo" nos textos e mostra o período (início–fim) que a API passa a devolver junto da
  chave.
- `docs/regras-de-calculo.md` é atualizado com a regra do ciclo, e um ADR curto em `docs/decisoes/` registra a troca.

## Detalhes decididos depois da revisão (minhas decisões, para conferir)

- **Nome do ciclo** vem do mês do dia **combinado** da 1ª parte, não do dia ajustado. Com `1=...`, se o dia 1 cair num
  domingo e o salário vier no dia 30 do mês anterior, o ciclo continua sendo o do mês do dia 1.
- **Lançamento com só `reference_month`** (recorrentes cadastrados à mão, como o financiamento) cai no ciclo **com o
  nome daquele mês**, não no dia 1 literal, que cairia no ciclo anterior.
- **Sobra e "veio do anterior"** usam os mesmos filtros de hoje (sem aplicações, resgates nem transferências próprias).
  No primeiro ciclo com dados, a linha não aparece.
- **Orçamento, metas e recorrentes** guardados por `AAAA-MM` continuam valendo: o mês N passa a ser o ciclo N, sem
  migração de dados.
- **Ritmo e "dias que faltam"** (Coach, Projeção, chat) contam até o fim do ciclo. Os investimentos usam o saldo no
  último dia do ciclo.
- **Tabela `cycles`** é regravada numa transação (apaga e insere juntos), então nenhuma consulta vê um estado pela
  metade. A janela vai do lançamento mais antigo até 2 anos à frente, sempre pela regra do salário atual. O fallback
  para o calendário só vale quando não há salário configurado.
- **API:** o `AAAA-MM` continua sendo a chave em todos os endpoints. Ganham `startsOn`/`endsOn` só as respostas que
  mostram o período na tela (Ciclo, Visão geral, Orçamento). O CSV filtra pelo ciclo com o mesmo parâmetro.

## Fora do escopo

- Escolher outro evento como início do ciclo (a fatura, por exemplo). O início é sempre o salário.
- Sobra acumulada desde sempre (saldo "virtual"). O saldo real já está no Painel.
- Guardar os ciclos antigos congelados quando o dia do salário muda.

## O que seria um fracasso

- A fatura paga com o salário de um ciclo aparecer em outro.
- Telas mostrando totais diferentes para o mesmo ciclo.
- O salário adiantado em um ou dois dias cair no ciclo anterior.
