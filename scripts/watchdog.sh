#!/bin/bash
# Watchdog do FinAssist: roda pelo LaunchAgent (scripts/launchd/) no login e a cada 2 minutos.
# `make watchdog-install` copia este arquivo para ~/Library/Application Support/finassist/ (o launchd
# não pode ler ~/Documents); depois de editar aqui, rode o install de novo.
# Garante que o Colima (VM do Docker) está de pé e que os containers do compose estão rodando.
# Idempotente: se já está tudo no ar, não faz nada.
#
# Para pausar (ex.: manutenção, `docker compose down` proposital): touch ~/.finassist-pause
# Para voltar: rm ~/.finassist-pause

export PATH="/opt/homebrew/bin:/opt/homebrew/sbin:/usr/bin:/bin:/usr/sbin:/sbin"
LOCK="/tmp/finassist-watchdog.lock"

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') $*"; }

[ -f "$HOME/.finassist-pause" ] && exit 0

# Evita duas execuções simultâneas (o colima start pode demorar mais que o intervalo).
if ! mkdir "$LOCK" 2>/dev/null; then
  # Lock velho (mais de 15 min) é de uma execução que morreu: remove e segue.
  if [ -n "$(find "$LOCK" -maxdepth 0 -mmin +15 2>/dev/null)" ]; then
    rmdir "$LOCK" 2>/dev/null; mkdir "$LOCK" 2>/dev/null || exit 0
  else
    exit 0
  fi
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT

# 1. Docker respondendo? Se não, sobe o Colima (o agente sh.brew.colima normalmente já cuida disso;
#    aqui é a rede de segurança para quando ele morre com erro e não é reiniciado).
if ! docker info >/dev/null 2>&1; then
  if ! colima status >/dev/null 2>&1; then
    log "colima parado; iniciando"
    colima start >/dev/null 2>&1 || { log "falha ao iniciar colima"; exit 1; }
  fi
  for _ in $(seq 1 60); do
    docker info >/dev/null 2>&1 && break
    sleep 2
  done
  docker info >/dev/null 2>&1 || { log "docker não respondeu"; exit 1; }
  log "docker disponível"
fi

# 2. Containers parados: religa pelo nome. Não usa o docker-compose.yml porque o launchd não tem
#    acesso a ~/Documents (TCC do macOS). Containers removidos (compose down) não voltam: aí é manual.
for c in $(docker ps -a --filter name=finassist- --filter status=exited --filter status=created --format '{{.Names}}'); do
  log "$c parado; iniciando"
  docker start "$c" >/dev/null 2>&1 || log "falha ao iniciar $c"
done

# 3. Container travado (unhealthy) não é reiniciado pelo Docker sozinho: reinicia aqui.
for c in $(docker ps --filter health=unhealthy --filter name=finassist- --format '{{.Names}}'); do
  log "$c unhealthy; reiniciando"
  docker restart "$c" >/dev/null 2>&1
done
