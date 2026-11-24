# Segunda instalação (finassist-bella): plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Objetivo:** permitir que uma segunda instalação do FinAssist, zerada, rode no mesmo Mac mini ao lado da atual.

**Arquitetura:** os nomes de container e as portas do host passam a ser variáveis no `docker-compose.yml`, e os padrões
são os valores de hoje. A instalação da Bella é um clone numa pasta irmã com `.env` próprio.

**Tecnologias:** Docker Compose, Make, shell.

**Spec:** `docs/superpowers/specs/2026-11-24-segunda-instalacao-design.md`

## Restrições globais
- Padrões: `CONTAINER_PREFIX=finassist`, `APP_PORT=3000`, `WEB_PORT=8080`, `POSTGRES_PORT=5432`, `EVOLUTION_PORT=8082`, `REDIS_PORT=6379`.
- Não definir `COMPOSE_PROJECT_NAME` por padrão. A instalação atual usa o nome da pasta (`go-financial-assistant`), e
  mudar isso trocaria o volume do banco dela.
- Bella: pasta `~/Documents/Projetos/finassist-bella`, `COMPOSE_PROJECT_NAME=finassist-bella`,
  `CONTAINER_PREFIX=finassist-bella`, `APP_PORT=3001`, `WEB_PORT=8081`, `POSTGRES_PORT=5433`.

## Foco da revisão
1. Com o `.env` atual, `docker compose config` não pode mudar em nada: mesmos nomes, portas e volumes.
2. O volume de banco da Bella não pode ser `go-financial-assistant_postgres_data`.
3. A instalação da Bella não pode reaproveitar `./secrets` nem `./backups` da atual.
4. O `docker compose up` da Bella não pode recriar nem parar os containers atuais.
5. O watchdog (`name=finassist-`) precisa pegar os containers `finassist-bella-*`.

---

### Tarefa 1: nomes e portas configuráveis

**Arquivos:**
- Alterar: `docker-compose.yml` (todos os `container_name` e as portas de host de `app`, `evolution` e `redis`)
- Alterar: `.env.example` (nova seção "Segunda instalação na mesma máquina")
- Alterar: `README.md` (seção curta com o passo a passo)

- [ ] Salvar `docker compose config` atual em `scratchpad/antes.yml`.
- [ ] Trocar `container_name: finassist-X` por `${CONTAINER_PREFIX:-finassist}-X`. Portas:
  `127.0.0.1:${APP_PORT:-3000}:3000`, `${EVOLUTION_PORT:-8082}:8080` e `127.0.0.1:${REDIS_PORT:-6379}:6379`.
- [ ] `docker compose config > depois.yml && diff antes.yml depois.yml`. Esperado: nenhuma diferença.
- [ ] Com `CONTAINER_PREFIX=finassist-bella APP_PORT=3001 WEB_PORT=8081 POSTGRES_PORT=5433 COMPOSE_PROJECT_NAME=finassist-bella`,
  `docker compose config` deve mostrar os nomes `finassist-bella-*`, as portas 3001/8081/5433 e o volume `finassist-bella_postgres_data`.
- [ ] Documentar no `.env.example` e no README. Commit `feat:` e push.

### Tarefa 2: criar a instalação da Bella

- [ ] `git clone` do repositório em `~/Documents/Projetos/finassist-bella`.
- [ ] `make init` lá (gera a chave em `./secrets` e o `SETUP_TOKEN` dela). Acrescentar ao `.env` dela as variáveis da
  restrição "Bella" e `CHANNEL=telegram`.
- [ ] `docker compose up -d --build`. Conferir `docker ps`: os containers atuais seguem `Up` desde antes, e os
  `finassist-bella-*` ficam healthy.
- [ ] `curl -s localhost:8081` responde, e `curl -s localhost:8080` continua respondendo.
- [ ] Repassar ao Thiago o `SETUP_TOKEN` dela (fica só no `.env` dela) e os próximos passos: BotFather, setup e Pluggy.
