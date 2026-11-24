# Segunda instalação do FinAssist (para a namorada)

## Por quê
Uma instalação própria, zerada, para a namorada do Thiago. É o mesmo app, mas ela tem bancos, bot do Telegram, senha,
banco de dados e backups próprios. Nada dela se mistura com a instalação atual, e as melhorias continuam chegando para as duas.

## Objetivos
- Rodar uma segunda instalação no mesmo Mac mini, ao lado da atual, sem conflito de nomes nem de portas.
- Banco vazio, `.env` próprio, chave mestra própria, pasta de backups própria.
- A instalação atual continua funcionando como hoje, sem precisar mudar o `.env` atual.

## Fora do escopo
- Vários usuários dentro de um mesmo app.
- Qualquer mudança no comportamento do app para ela.

## Anti-objetivos
- Copiar dados, segredos ou backups da instalação atual para a dela.
- Mexer na instalação atual (containers, volume do banco, Tailscale) sem combinar antes.

## Abordagens
1. **Segunda pasta (clone do repositório), escolhida.** Cada instalação tem a própria pasta, com `.env`, `./secrets` e
   `./backups` próprios. O isolamento é total, e ela atualiza com `git pull`.
2. A mesma pasta com um segundo `.env`: descartada, porque `./backups` e `./secrets` ficariam compartilhados.
3. App com vários usuários: descartada, porque seria uma reescrita e o banco seria um só para os dois.

## Como fica
- No `docker-compose.yml`:
  - `container_name` vira `${CONTAINER_PREFIX:-finassist}-<serviço>`.
  - Ganham variáveis com padrão a porta do app no host (`APP_PORT:-3000`), a do Evolution (`EVOLUTION_PORT:-8082`) e a
    do Redis (`REDIS_PORT:-6379`).
  - `POSTGRES_PORT` e `WEB_PORT` já são variáveis.
  - Os padrões são os valores de hoje, então a instalação atual não muda.
- No `.env.example`: documentar `COMPOSE_PROJECT_NAME`, `CONTAINER_PREFIX`, `APP_PORT`, `POSTGRES_PORT` e `WEB_PORT` numa
  seção "Segunda instalação na mesma máquina".
- No README: uma seção curta com o passo a passo.
- A instalação dela, numa pasta irmã: `COMPOSE_PROJECT_NAME=finassist-<nome>`, `CONTAINER_PREFIX=finassist-<nome>`,
  `APP_PORT=3001`, `WEB_PORT=8081`, `POSTGRES_PORT=5433`. Depois, `make init` e `docker compose up -d`.
  O volume do banco ganha o prefixo do projeto e nasce vazio.
- O watchdog já religa qualquer container `finassist-*`, então cobre os containers dela sem mudança.

## Fica para depois / para eles
- Criar o bot dela no @BotFather, fazer o setup no navegador e conectar os bancos dela no Meu Pluggy.
- Acesso pelo celular: um `tailscale serve` em outra porta HTTPS apontando para a 8081. Combinar antes de rodar.
