# Regras de cálculo

Tudo é calculado por código (nada de IA nos números). Valores em reais; um mês é identificado pelo seu primeiro dia (data, no fuso configurado em `DIGEST_TIMEZONE`, padrão America/Sao_Paulo). O fuso decide o "hoje" e o "este mês" (recorrências, CSV e relatório mensal, consultas do chat, painéis) e o mês dos lançamentos manuais avulsos, que não têm vencimento e usam o instante em que foram anotados: um gasto das 22h do dia 30 conta no mês do dia 30, não no seguinte. O banco recebe o mesmo fuso na sessão; trocá-lo nas configurações vale depois de reiniciar.

**Recorrentes (despesa e renda):** cada recorrente ativa é lançada uma vez por mês no dia programado (dia 29, 30 ou 31 cai no último dia dos meses mais curtos). Se o app estava fora nesse dia, ela é lançada na subida ou no próximo ciclo da meia-noite do mesmo mês, com o mesmo valor e mês e a data do dia programado; um mês que já acabou não é lançado depois. Um mês que já tem qualquer pagamento dela, mesmo cancelado, não recebe outro; um pagamento do mês apagado à mão, depois do dia programado, volta a ser lançado na próxima subida (para não lançar, cancele a recorrente). Recorrente cancelada não é lançada.

## O que entra como gasto ou renda (sincronização)

Feito em `infra/pluggy/mapper.go`. A transação do Pluggy vira lançamento, salvo quando é:

- **Transferência entre contas suas**: categoria `Same person transfer` do Pluggy, ou Pix/TED/DOC com um dos nomes de `OWN_NAMES` (`domain.OwnTransferMatcher`). Só move dinheiro de lugar.
- **Pagamento de fatura de cartão**: categoria `Credit card payment` **e** descrição com cara de fatura (fatura, cartão, bandeira, "pagamento recebido"). O Pluggy rotula *todo* boleto pago assim; boleto para um banco (ex.: parcela de financiamento) e Pix QR Code são gasto de verdade.
- **Crédito em cartão** (estorno ou pagamento): não é renda.
- **Aplicação automática do Itaú** ("APLIC AUT MAIS"): varre o saldo para um CDB e de volta; os rendimentos pagos continuam sendo renda.

**Revisão de transferências:** as transferências do banco (categoria `Transfer ...` do Pluggy) esperam a sua decisão na tela Classificar. "Entre contas minhas" cancela o pagamento (`CANCELLED`: deixa de contar em tudo, como a limpeza do `OWN_NAMES`; não vira `TRANSFER`, que é de investimento); "Para uma pessoa" muda a categoria para Pessoas; "Pagamento" muda para a categoria escolhida (só saídas). O tipo (despesa ou renda) nunca muda. Desfazer só reverte o que a decisão mudou, e só se ninguém mexeu depois: "Entre contas minhas" volta ao status anterior se o pagamento ainda está `CANCELLED`; "Para uma pessoa" e "Pagamento" voltam à categoria anterior se a atual ainda é a que a decisão aplicou (`applied_category`) e nunca mexem no status. Nada é decidido sozinho.

Aplicações e resgates de investimento viram `TRANSFER` (aparecem em Investimentos, não em gastos). A categoria vem do texto do Pluggy e, quando fica em Outros, de regras por descrição.

Algumas categorias do Pluggy casam pela categoria **inteira**, não por trecho (Housing → Moradia; Utilities, Telecommunications, Loans and financing e Services → Contas; Bookstore → Educação; Gyms and fitness centers → Saúde). Ao fim de cada sincronização, as despesas do banco que estão em Outros (também as mais antigas que a janela de datas) são reclassificadas pela categoria gravada em `raw_input`; só a categoria muda, e regras de categoria por conta (inclusive "manter em Outros") sempre valem.

### Parcelas no cartão

Cada parcela é datada no **mês da fatura em que cai** (`creditCardMetadata.billForecastDate`), mantendo o dia da compra. O Pluggy dá a mesma data de compra a todas as parcelas; sem o ajuste, as 12 parcelas caíam no mês da compra. Parcelas ainda não cobradas ficam `PENDING`.

### Mês das compras no cartão (mês da fatura)

Como na planilha, **toda despesa no cartão conta no mês em que a fatura vence** (`payments.bill_due_date`, calculada em `domain/card_bill.go`); o dia mostrado nas listas continua sendo o da compra. O mês de um pagamento é `COALESCE(bill_due_date, due_date, reference_month, created_at)` (`paymentMonth`), em todas as telas, no chat, no CSV, na projeção e no orçamento.

- **Fatura informada pelo banco (vale primeiro):** o Pluggy diz em que fatura cada transação do cartão entrou (`creditCardMetadata.billId`, gravado em `payments.card_bill_id`). Quando essa fatura está entre as faturas fechadas lidas (`card_bills`), vale o vencimento dela, sem estimativa: o fechamento real varia (no Santander, de 5 a 8 dias antes do vencimento, conforme fins de semana). As regras abaixo valem para o que não tem fatura informada (a fatura aberta e conectores sem `billId`).
- **Pendente perto do fechamento:** num cartão em que o banco informa a fatura das transações, compra à vista sem fatura informada que a estimativa poria na última fatura fechada vai para a seguinte (ela ficou pendente no fechamento e o banco a passou adiante).
- **Vencimento:** o dia do vencimento do cartão informado pelo banco (`accounts.due_date`); dia 29 a 31 vira o último dia nos meses mais curtos.
- **Fechamento:** o informado pelo banco, quando houver (vencimento − fechamento); senão, `CARD_CLOSE_DAYS_BEFORE_DUE` dias antes do vencimento (padrão 6; o Open Finance raramente informa).
- **Compra à vista:** vai para a primeira fatura cujo fechamento é **depois** do dia da compra. Comprou no dia do fechamento ou depois, vai para a seguinte (o "melhor dia de compra" é o próprio fechamento). Ex.: vencimento dia 10, fecha dia 4: compra em 03/11 → fatura de 10/11; em 04/11 ou 20/10 → 10/12 e 10/11.
- **Parcela:** a fatura do mês em que a sincronização já a datou (o do banco).
- **Quando é calculado:** ao fim de cada sincronização, para todas as despesas dos cartões com vencimento conhecido (inclusive as antigas), e na hora em que o fechamento muda nas configurações. Só grava o que mudou. Lançamentos manuais no cartão (pelo chat) não têm cartão e ficam no mês do dia anotado.

### Dinheiro seu mudando de lugar

Ao fim de cada sincronização (`ReclassifyOwnMoves`, idempotente, vale também para o que já estava gravado):

- **Caixinha do Mercado Pago** ("Dinheiro reservado …" / "Dinheiro retirado …"): vira aplicação e resgate (transferência de investimento), não gasto nem renda.
- **Pagamento que o banco chama de aplicação** ("Pagamento com Pix …", "Pagamento de boleto …", "Pagamento de conta …" com categoria de investimento, ex.: Pix para um FIDC): volta a ser gasto, em Outros.
- **Pagamento da fatura de um cartão seu por uma conta** (a descrição traz o nome do cartão, ex.: "Pagamento de conta SANTANDER UNIQUE VISA INFINITE"): fica cancelado, com o motivo registrado, porque as compras do cartão já contam no mês da fatura. Só casa nome de cartão com 10 letras ou mais que não seja também nome de conta corrente.

### Notas das transações

Cada transação pode ter uma **nota** sua (`payments.note`, até 300 caracteres), escrita em Transações ou pelo chat ("nota <valor> [dia] <texto>"). A sincronização nunca mexe na nota. Pelo chat, a transação é achada pelo valor (de qualquer sinal) no dia dito; sem resultado, ±3 dias; sem dia, nos últimos 60 dias. Nota de transação que ainda não chegou fica em `pending_notes` e é aplicada ao fim de cada sincronização quando há **exatamente uma** transação sem nota com o valor, a até 3 dias do dia dito (na dúvida, espera); some depois de 60 dias.

### Estornos no cartão

Crédito no cartão que não é o pagamento da fatura (categoria "Credit card payment" do Pluggy ou "pagamento" na descrição) é **estorno**: entra como **despesa com valor negativo**, na categoria e na fatura do crédito, e abate o mês como no extrato do banco ("Total de créditos"). Só a sincronização grava valor negativo; lançamentos manuais continuam positivos, e o estorno nunca é conciliado com um deles.

### Parcelas previstas e ajuste da fatura

O Open Finance entrega as parcelas com atraso (a fatura já fechada pode não ter nenhuma) e nem sempre todos os lançamentos da fatura. Ao fim de cada sincronização o app refaz, numa transação, dois tipos de lançamento próprios nos cartões (`external_id` começando com `finassist:`; nunca editados, sempre apagados e recriados; ver `domain/card_forecast.go` e `infra/db/card_forecast_repository.go`):

- **Parcela prevista** (`PENDING`, mesma descrição "BASE (n/m)", mesma categoria e valor): cada compra parcelada é reconhecida por cartão, descrição, número de parcelas, valor e mês de início (fatura menos a parcela); a partir da maior parcela que chegou, as seguintes, uma por fatura, **até a fatura aberta hoje** (e nunca além da última fatura com algum lançamento real do cartão: cartão parado não ganha previsão). Só se prevê **depois da última fatura em que o banco entregou alguma parcela** daquele cartão, e só para as compras que tinham parcela nela (as que pararam antes terminaram ou mudaram de nome no banco). Compras iguais no mesmo mês contam como cópias. Quando a parcela real chega, a prevista some na sincronização seguinte. A projeção continua prevendo as faturas mais distantes pela parcela mais recente (inclusive prevista), sem contar duas vezes.
- **Ajuste da fatura** (`PAID`, categoria Outros, "Fatura <cartão> dd/mm · não detalhado no Open Finance"): para cada fatura fechada que o banco informa (`card_bills`, `GET /bills` do Pluggy) a partir do começo do histórico do cartão, total da fatura − (lançamentos reais + previstos do cartão naquela fatura). Só quando a diferença é de R$ 0,50 ou mais, nos dois sentidos: positiva ("não detalhado": compra de cartão adicional que o Open Finance não manda, saldo que sobrou da fatura anterior) ou negativa ("Fatura <cartão> dd/mm · créditos não detalhados no Open Finance", gravada como estorno). Assim o mês da fatura fechada sempre bate com o banco.

### Chave de conta (`expenseKey`)

Para reconhecer a mesma conta em meses diferentes, a descrição é normalizada: minúsculas, só letras (some número, data, parcela, pontuação), espaços compactados. Compras no débito do Itaú ("DEBITO VISA ELECTRON BRASIL dd/mm COMERCIO") usam o prefixo mais a primeira palavra do comércio, porque o nome muda de grafia a cada mês. Regras salvas (fixa/variável, categoria, dispensas, decisões) usam essa chave.

## Orçamento (`planning/budget.go`)

Cada conta (chave) é classificada como **fixa**, **parcelada** ou **variável**, nesta ordem: correção manual, parcelada (`n/m` na descrição ou tipo), recorrente cadastrada, detecção de conta fixa e, por fim, variável. A classe vale para a conta inteira na janela de 12 meses.

**Detecção de fixa:** aparece em pelo menos 3 meses da janela (ou em todos, havendo só 2), no máximo 2 vezes por mês, com valor parecido: (maior − menor) / mediana dos totais mensais ≤ 30% (≤ 5% com só 2 meses de dados, porque o acaso é mais provável).

## Projeção (`planning/projection.go`)

Projeta N meses (6, 12 ou 24) a partir do mês atual:

- **Fixas e variáveis**: média dos últimos 3 meses completos com dados (editável na tela).
- **Renda**: mediana da renda total dos últimos 6 meses (um mês fora do padrão — 13º, adiantamento de férias — não pesa). As fontes recorrentes (`EstimateIncome`) explicam a maior parte; o restante aparece como "outras entradas". **Todo lançamento da categoria Salário é uma fonte só**, mesmo que a descrição mude. Com o **salário configurado** (`SALARY_SCHEDULE`), ele entra pelo valor combinado de cada parte ("Salário do dia 15", "Salário do dia 30") no lugar da estimativa, e a mediana e as fontes recorrentes passam a olhar só as outras entradas.
- **Parcelas**: as que faltam das compras parceladas (inferidas do `n/m` da descrição e das já cadastradas).
- **Saldo projetado** = renda − fixas − variáveis − parcelas, acumulado.

É uma estimativa: gastos novos, reajustes e rendimentos não entram.

## Revisão de gastos (`review/review.go`)

Sugestões de corte, sempre com a economia em R$/mês e R$/ano:

| Tipo | Regra |
| --- | --- |
| Aumento | categoria subiu ≥ R$ 50 e ≥ 20% sobre a média dos até 3 meses anteriores |
| Fixa / assinatura | conta fixa detectada; economia anual = valor × 12 |
| Gasto formiga | ≥ 4 compras de até R$ 40 (fora Pix) somando ≥ R$ 80 na mesma conta; economia = metade |
| Possível duplicata | mesmo valor e conta em até 3 dias (≥ R$ 5) |
| Conta nova | só aparece no mês, com ≥ 2 meses anteriores de dados (≥ R$ 30) |

Duplicata e conta nova são avulsas (não anualizadas). Sugestões podem ser **dispensadas** ou marcadas **"Cancelei"** (só fixa e formiga).

### Economia realizada (`review/savings.go`)

Para cada "cancelei", confere mês a mês, do mês seguinte à decisão até hoje, se a cobrança sumiu: **confirmado** (sumiu), **voltou** (o mês trouxe ≥ metade do custo) ou **pendente** (nenhum mês fechado ainda). Economia = custo mensal × meses confirmados. O mês em andamento só serve para flagrar a volta.

## Metas (`planning/goals.go`)

O progresso é calculado na hora. Patrimônio = saldo das contas correntes + investimentos.

- **Juntar valor até uma data**: quanto guardar por mês; "cabe" compara com a sobra média projetada.
- **Reduzir uma categoria em X%**: teto = média dos até 3 meses anteriores à criação (fixada na criação); acompanha cada mês desde então e o ritmo do mês atual (a partir do dia 7).
- **Reserva de N meses**: alvo = N × despesas fixas da projeção; mostra quantos meses o patrimônio cobre.

## Saldos (`balances/`)

- **Total em conta** = soma do saldo das contas `BANK` (negativo entra com sinal). **Participação** do banco = saldo ÷ soma dos saldos positivos (banco negativo não ganha fatia).
- **Fatura das compras de hoje:** cada cartão mostra em que fatura entram as compras feitas agora e quando ela fecha, pela mesma regra do mês das compras; com "~" quando o fechamento é estimado.
- **Cartão** (nunca somado entre cartões): o valor mostrado é o **saldo devedor** informado pelo banco = limite usado (inclui parcelas futuras; não é a fatura aberta, que o Open Finance não expõe). Uso do limite: ≥ 70% atenção, ≥ 90% crítico. Vencimento em até 3 dias: atenção; data já passada não alerta (o banco mantém o vencimento da fatura paga).
- **Desatualizado**: mais de 36 h desde a última atualização *do Pluggy* (`lastUpdatedAt` do item), não desde a última cópia do app. O Meu Pluggy não permite forçar a atualização pela API; ela acontece sozinha (cerca de 1x/dia) ou a pedido em meu.pluggy.ai.
- O "aplicado automaticamente" do banco só aparece se for diferente do saldo.
- O nome e a cor do banco vêm do conector; no Meu Pluggy o conector é sempre o agregador, então vêm do nome das contas (tabela de bancos conhecidos).

## Salário e dias úteis (`domain/salary.go`, `domain/business_day.go`)

- **Partes** em Configurações → Renda (`SALARY_SCHEDULE`), como `15=5200; 30=4366,62`. Vazio desliga.
- **Data de cada parte:** o dia combinado; se passar do fim do mês, o último dia (30 em fevereiro = 28 ou 29); se não for dia útil, o **dia útil anterior**.
- **Dia útil:** segunda a sexta, menos os feriados nacionais (1/1, 21/4, 1/5, 7/9, 12/10, 2/11, 15/11, 20/11, 25/12), os bancários móveis (Carnaval segunda e terça, Sexta-feira Santa, Corpus Christi, pela Páscoa) e os de São Paulo (25/1 e 9/7).
- **Conferência com o banco:** cada parte fica com o crédito da categoria Salário mais perto da data prevista, até 5 dias de distância; um crédito serve a uma parte só, e créditos extras (férias, 13º) ficam de fora. Parte sem crédito = **a receber**. Nada é lançado: o salário real continua vindo só do Open Finance, então não duplica.
- **Painel:** mostra as partes do mês, quando caem ou caíram e quanto falta receber. Parte que veio acima do combinado (hora extra no mesmo crédito) mostra a diferença. Créditos de salário do mês que não ficaram com nenhuma parte (13º, PLR, férias, hora extra à parte) aparecem como **extras**; eles contam nas entradas do mês, mas não no "a receber" nem na projeção, que usa só o combinado.

## Tela do mês (`planning/month_sheet.go`)

O mês como na planilha (`GET /api/month`, página **Mês**), pelo mesmo critério de mês das outras telas (compra no cartão no mês da fatura):

- **Entradas:** o que entrou no mês, somado pelo nome; todo crédito da categoria Salário vira uma linha "Salário". **A receber:** as partes do salário configurado que ainda não caíram, com a data no dia útil.
- **Despesas fixas:** fora do cartão, as contas que o orçamento classifica como fixas ou parceladas (a mesma classe da tela Orçamento, inclusive as correções "é fixa / não é fixa").
- **Fatura de cada cartão:** o total do mês da fatura, por categoria (compras menos estornos), mais as parcelas previstas e o ajuste pelo total do banco em linhas separadas. Lançamento manual no cartão (sem conta) entra como "Cartão (lançado à mão)".
- **Outras despesas:** o resto fora do cartão, por categoria; o que está em Outros aparece pelo nome (Pix para uma pessoa, um boleto).
- **Sobra** = entradas − (fixas + faturas + outras). Com salário a receber, mostra também a sobra contando com ele. Aplicações e resgates ficam fora da sobra e aparecem à parte.

## Mês em andamento

**Aviso de "Outros" alto:** a Visão geral avisa quando as despesas em `OTHER` passam de **30%** das despesas do mês (exatamente 30% não avisa; mês sem despesas não avisa). A conta é a do total de despesas (só despesa, sem pagamentos cancelados) e é feita em código, sem IA; o aviso leva a Classificar (contas em Outros e transferências a revisar).

Valores do mês atual são parciais. As telas que comparam com o mês anterior avisam, e a Visão geral não mostra a variação enquanto o mês não fecha.

## Investimentos

O saldo vem do Pluggy (`/investments`, líquido de impostos) e é gravado a cada sincronização, formando o histórico do patrimônio. **Do mês da primeira sincronização em diante** o saldo ao fim de cada mês é exato (soma do último saldo gravado de cada posição). **Antes disso é estimado**, por posição: saldo do primeiro registro menos o que foi aplicado depois do mês (e antes desse registro), sem contar rendimentos; posição sem movimentação até o fim do mês ainda não existia e fica de fora; o saldo nunca é negativo. Posições que somem do Pluggy (resgatadas) ficam inativas com saldo zero. O saldo é gravado no **dia civil do fuso configurado** (`DIGEST_TIMEZONE`), então uma sincronização às 22h do último dia do mês conta nesse mês; saldos gravados antes dessa regra (dia UTC) não foram corrigidos.

## Conciliação com lançamentos manuais

Ao sincronizar, uma transação do banco que ainda não existe é conciliada com um lançamento manual equivalente (mesmo tipo e valor): avulso, com até 3 dias de diferença; recorrente, no mesmo mês (a recorrência é gerada em dia fixo e o banco pode atrasar). Parcelados ficam de fora porque a data de cada parcela no cartão não é previsível. Se achar, vincula (e marca como pago); senão cria um lançamento novo.
