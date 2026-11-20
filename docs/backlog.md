# Backlog

Mantido pelo PO. **Prioridade proposta, aguardando OK do Thiago.** Nada daqui vai ao Tech Lead sem esse OK.
Cada história cabe em uma branch. Origem das 5 primeiras: Discovery (dados agregados, 11/09 a 10/11/2026).

| # | História | Valor | Esforço | Risco | Prioridade proposta | Status |
| --- | --- | --- | --- | --- | --- | --- |
| H1 | Categorias novas na sincronização do Pluggy | alto | pequeno | baixo | 1 | Entregue na branch (QA ok), falta merge do Thiago |
| H2 | Revisar transferências após a sincronização | alto | médio | médio | 2 | Aprovada pelo Thiago (quebrar, veja abaixo) |
| H3 | Tarefas agendadas no fuso configurado | médio | pequeno | baixo | 3 | Aprovada pelo Thiago |
| H4 | Recorrente recupera o mês perdido | médio | pequeno | baixo | 4 | Entregue e enviada (QA ok) |
| H5 | Migrations aplicadas ao subir o app | médio | médio | médio | 5 | Aprovada pelo Thiago |
| H6 | Saldo de investimentos no dia certo | baixo-médio | pequeno | baixo | 6 | Aprovada pelo Thiago (origem: Tech Lead) |
| H7 | Corrigir teste instável do Coach (CI) | baixo | pequeno | baixo | 7 | Coberta pela correção técnica do Tech Lead (só testes) |
| H8 | "Cancelar X" pergunta quando há mais de uma recorrente parecida | médio | pequeno | baixo | 8 | Proposta (origem: Tech Lead), aguardando OK |
| H9 | Separar cartão de Pix/TED/DOC nas telas | alto | médio | médio | 9 | Proposta, aguardando OK |
| H10 | Esconder movimentações do Cofrinho do Mercado Pago | alto | pequeno | médio | 10 | Proposta, aguardando OK |
| H11 | Erros intermitentes do Telegram (getUpdates) | médio | pequeno | baixo | 11 | Proposta (origem: Sustentação), aguardando OK |

Ordem sugerida: H1 é rápida e já melhora os totais; H2 é o maior ganho, mas depende de decisões suas (ver perguntas); H3 antes de H4 porque as duas mexem em datas e a H4 depende do fuso correto.

---

## H1. Categorias novas na sincronização do Pluggy
**Como** dono, **quero** que aluguel, contas, financiamento, telecom e mensalidades cheguem já em Moradia, Contas, Educação etc., **para** não ficarem em "Outros".

Contexto: o CHANGELOG diz que Moradia, Contas, Educação e Pessoas existem, mas o mapeamento (`backend/internal/infra/pluggy/mapper.go`, ~linhas 56-61) não as usa. Ficaram em "Outros" (~R$ 16 mil, 32 lançamentos): Utilities, Loans and financing, Telecommunications, Bookstore, Gyms and fitness centers, Services, Housing.

Critérios de aceite:
- Uma despesa nova do Pluggy com categoria Housing cai em Moradia; Utilities e Telecommunications em Contas; Bookstore em Educação. Decisão do Thiago (delegada ao PO): Loans and financing em Contas, Gyms and fitness centers em Saúde, e Services em Contas.
- A categoria do Pluggy, quando útil, continua valendo mais que as regras por palavra (regra já existente).
- Após uma sincronização, as despesas já gravadas em "Outros" com essas categorias são reclassificadas.
- Regras de categoria que o Thiago criou à mão não são sobrescritas.
- Teste automatizado cobre cada mapeamento novo; nenhuma outra categoria muda.
- Os totais por categoria no dashboard refletem a mudança sem alterar o total geral de despesas.

Riscos: quebra de função existente (reclassificação em massa muda gráficos, Orçamento e metas de redução por categoria). Mencionar no CHANGELOG.

---

## H2. Revisar transferências após a sincronização
**Como** dono, **quero** confirmar o que cada transferência é (entre contas minhas, para pessoa ou pagamento), **para** os totais de despesa e renda não serem inflados.

Contexto: 90,9% do valor das despesas está em "Outros"; ~R$ 151 mil de ~R$ 168 mil vêm de 29 transferências "Transfer - PIX" / "Transfer - Internal". Nas entradas, ~R$ 146 mil. Hoje o `mapper.go:19` só ignora "same person transfer". Nenhuma saída tem entrada de mesmo valor em outra conta conectada (0 de 29), então não dá para ignorar sozinho.

Critérios de aceite:
- Após cada sincronização, as transferências ainda não resolvidas aparecem numa lista "Transferências a revisar" (reaproveitar a tela Classificar, se couber).
- Para cada uma, três escolhas: **entre contas minhas** (sai dos gastos e da renda, vira Transferência), **para pessoa** (categoria Pessoas) ou **pagamento** (escolher categoria).
- Nada é decidido sem a confirmação do Thiago.
- Toda decisão pode ser desfeita, e a transferência volta para a lista.
- A decisão vale para os próximos casos da mesma contraparte, se o Thiago marcar essa opção.
- Pix entre contas minhas deixa de entrar em despesas e renda (Visão geral, Gastos, Projeção).
- A Visão geral mostra um aviso quando "Outros" passar de X% das despesas do mês (X = 30%, decidido pelo Thiago) e leva à lista.
- Nenhum nome de pessoa é enviado à IA.

Riscos: altera totais e a renda da Projeção (mediana de 6 meses). Quebrar em branches menores:
- H2a: lista e as 3 decisões, com desfazer.
- H2b: aprender com a decisão para próximas sincronizações.
- H2c: aviso de "Outros" alto na Visão geral.

Nota: o Thiago acabou de preencher "Seus nomes nos extratos" (`OWN_NAMES`). Isso já deve reduzir a lista; medir de novo após a próxima sincronização antes de começar a H2, pois o tamanho real do problema pode ter diminuído.

---

## H3. Tarefas agendadas no fuso configurado
**Como** dono, **quero** que o CSV mensal e os lançamentos recorrentes usem o meu fuso, **para** o CSV não perder as últimas horas do mês nem a recorrente cair no dia errado.

Contexto: `app/jobs.go:37-55` e `ledger/monthly_report.go:52` usam meia-noite UTC. O CSV "do dia 1" sai às 21h do último dia do mês em Brasília e deixa de fora as últimas 3 horas.

Critérios de aceite:
- O fuso é o mesmo de `DIGEST_TIMEZONE` (padrão `America/Sao_Paulo`).
- O CSV do mês anterior cobre de 00:00 do dia 1 a 23:59:59 do último dia, no fuso configurado; uma transação às 22h do último dia entra.
- Os envios do dia 1 e as recorrentes disparam no dia local, não no UTC.
- Teste com relógio fixo (`domain.Clock`) cobre a virada do mês e o horário 21h-23h59.
- Sem mudança de comportamento para quem já está em UTC.

---

## H4. Recorrente recupera o mês perdido
**Como** dono, **quero** que uma despesa recorrente seja lançada mesmo se o app estava fora no dia, **para** o mês não ficar sem a conta.

Contexto: `usecase/ledger/analyze_expense.go:174-178` só lança no dia exato.

Critérios de aceite:
- Se o dia da recorrente já passou e o mês não tem pagamento dela, o app lança ao subir ou no próximo ciclo.
- A data do lançamento é a do dia programado (não a de hoje).
- Nunca duplica: com pagamento no mês (`HasPaymentForMonth`), não lança.
- Recorrente cancelada ("cancelar netflix") não é recuperada.
- Dias 29 a 31 em meses curtos seguem a regra atual.
- Teste cobre: app fora no dia, app no dia, recorrente cancelada e duplicata.
- Depende da H3 (fuso).

---

## H5. Migrations aplicadas ao subir o app
**Como** dono, **quero** que o banco existente receba as migrations novas sozinho, **para** não depender do passo manual do README.

Contexto: `docker-compose.yml:13` só roda as migrations na criação do banco. O laço do README (`01[0-9]_*.sql`) ignora a 020 em diante. Se faltar uma, o app só avisa no log.

Critérios de aceite:
- Uma tabela de controle registra as migrations já aplicadas.
- Ao subir, o app aplica as pendentes em ordem, cada uma uma vez.
- Antes de aplicar, é feito um backup (reaproveitar o serviço `backup`); sem backup, não aplica e avisa.
- Em banco que já tem as migrations aplicadas à mão, o app reconhece o estado atual sem reaplicar nem quebrar (as existentes são idempotentes).
- Falha numa migration interrompe a subida com mensagem clara e não deixa o banco pela metade (transação).
- README e CHANGELOG deixam de mandar aplicar à mão.
- Decisão do Thiago: aplicação automática (não só o aviso).

Riscos: mexe em dados reais; testar primeiro numa cópia restaurada do backup.

---

## H6. Saldo de investimentos no dia certo
**Como** dono, **quero** que o saldo dos investimentos fique registrado no dia local da sincronização, **para** o patrimônio de fim de mês não perder o último registro.

Contexto (Tech Lead): uma sincronização entre 21h e 24h do último dia do mês grava o saldo no dia 1 do mês seguinte, porque usa o dia UTC (`openfinance/sync.go:147`). Já existia antes da H3.

Critérios de aceite:
- O saldo gravado usa o dia no fuso configurado (`DIGEST_TIMEZONE`).
- Uma sincronização às 22h do último dia do mês entra no patrimônio daquele mês.
- Teste com relógio fixo cobre a virada do mês.
- Saldos já gravados no dia errado: decidir com o Thiago se são corrigidos ou só valem daqui para frente.

Status: aprovada pelo Thiago, repassada ao Tech Lead. Saldos antigos: só valem daqui para frente (padrão do PO, o Thiago respondeu só "sim"; ele pode pedir a correção depois).

---

## H7. Corrigir teste instável do Coach
**Como** dono, **quero** que o teste da API do Coach não falhe por acaso, **para** o CI não quebrar sem motivo.

Contexto (Tech Lead): `backend/internal/infra/http/coach_api_test.go:152` procura "999" no corpo inteiro da resposta, e o UUID aleatório às vezes contém "999". Falhou 1 a 2 vezes em 150 execuções.

Critérios de aceite:
- O teste verifica só o campo `advice`, não o corpo inteiro.
- 300 execuções seguidas passam sem falha.
- Nenhuma mudança de comportamento do app.

Status: o Tech Lead abriu uma correção técnica só de testes que cobre esta história, sem precisar de aprovação de produto.

---

## H8. "Cancelar X" pergunta quando há mais de uma recorrente parecida
**Como** dono, **quero** que o bot me pergunte qual recorrente cancelar quando mais de uma combinar com o texto, **para** não cancelar a errada.

Contexto (Tech Lead): hoje "cancelar conta" com "Conta de luz" e "Conta de água" ativas cancela a primeira em ordem alfabética, sem perguntar.

Critérios de aceite:
- Com uma só recorrente combinando, cancela como hoje.
- Com mais de uma, o bot lista as opções (nome e valor) e só cancela a que o Thiago escolher.
- Sem resposta ou com "nenhuma", nada é cancelado.
- Teste cobre: uma correspondência, várias, nenhuma e escolha inválida.

Status: proposta, aguardando OK do Thiago. Não repassada ao Tech Lead.

---

## H9. Separar cartão de Pix/TED/DOC nas telas
**Como** dono, **quero** ver compras no cartão separadas de Pix, TED e DOC, **para** entender cada tipo de gasto sem misturar.

Contexto: hoje há filtro por forma de pagamento em Transações e a divisão em Gastos, mas a separação não é o padrão em nenhuma tela. A tela exata (Transações, Gastos, Visão geral) está por confirmar com o Thiago.

Critérios de aceite:
- Transações e Gastos mostram dois grupos claros: **Cartão** (crédito e débito) e **Transferências** (Pix, TED, DOC), cada um com seu total do mês.
- O filtro por grupo fica na URL, como os demais filtros.
- Pix/TED/DOC entre contas suas (transferência própria) continuam fora dos totais de despesa.
- A soma dos dois grupos mais os demais meios (boleto, dinheiro) bate com o total de despesas do mês.
- Nada muda no cálculo dos totais, só no agrupamento e na apresentação.
- Teste cobre um mês com cartão, Pix, TED, DOC e boleto.

Perguntas ao Thiago: em quais telas? E débito entra em Cartão, como proposto?

---

## H10. Esconder movimentações do Cofrinho do Mercado Pago
**Como** dono, **quero** que aplicações e resgates do Cofrinho do Mercado Pago não apareçam como movimentações, **para** a lista e os totais mostrarem só gasto e renda reais.

Contexto: o README diz que aplicações e resgates viram Transferência, mas as do Cofrinho do Mercado Pago estão aparecendo. Hoje o app só trata "APLIC AUT MAIS" do Itaú. Falta confirmar, por consulta de leitura em agregados, como o Pluggy descreve essas movimentações.

Critérios de aceite:
- Entradas e saídas do Cofrinho do Mercado Pago não aparecem em Transações, Gastos nem na renda.
- Rendimentos pagos pelo Cofrinho continuam entrando como receita (confirmar com o Thiago).
- O saldo do Cofrinho continua em Investimentos, se o Pluggy o fornece.
- Lançamentos antigos já gravados são tratados na próxima sincronização, sem apagar dados; decisão reversível.
- Sem efeito nas movimentações de outros bancos.
- Teste cobre aplicação, resgate e rendimento.

Risco: o Pluggy pode não distinguir o Cofrinho de uma transferência comum; nesse caso a regra precisa de uma marca confiável na descrição.

Pergunta ao Thiago: rendimento do Cofrinho entra como receita ou também fica escondido?

---

## H11. Erros intermitentes do Telegram (getUpdates)
Origem: Sustentação, rotina de 11/11. Cerca de 25 erros em 10 h (502 em rajada, depois timeouts e "unexpected EOF"). Containers saudáveis. Possível atraso ou perda pontual de mensagens do bot.

Critérios de aceite:
- O Tech Lead avalia o timeout do cliente e do long-polling e a nova tentativa, e decide a correção.
- O erro intermitente não gera ruído de ERROR a cada ocorrência; só avisa se falhar por um período longo.
- Nenhuma mensagem do bot se perde por causa de uma falha de polling.

Atualização H11 (Sustentação, 12/11): o problema continua. Cerca de 40 ERROR entre 11/11 15:39 e 12/11 08:30, quase todos timeout, 5 "unexpected EOF". Recorrente há mais de 24 h, não se resolveu sozinho.
Atualização H11 (Sustentação, 13/11): 34 ERROR de 12/11 09:19 a 13/11 08:02 (timeouts, 4 "unexpected EOF", 3 "502" em 12/11 22:11). Terceiro dia seguido; não se resolveu sozinho.
Atualização H11 (Sustentação, 14/11): 4º dia. 14 ERROR (13 timeouts, 1 EOF), sem 502, de 13/11 09:30 a 14/11 07:49. Frequência menor, mas persiste.
Atualização H11 (Sustentação, 15/11): 5º dia e piorou. 90 ERROR de 14/11 09:18 a 15/11 06:55 (50 "502", 38 timeouts, 2 EOF). Prioridade sugerida: subir para 1.
