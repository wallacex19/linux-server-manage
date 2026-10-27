#!/usr/bin/env bash
# ==============================================================================
# Linux Server Manager - Privileged Helper
# Executado com privilégios de root restritos via /etc/sudoers.d/lsm-agent
# Permite exclusivamente operações tipadas de serviços systemd e logs journald.
# NÃO permite NOPASSWD: ALL nem execução de comandos arbitrários ou kill de processos.
# ==============================================================================

set -euo pipefail

log_err() {
    echo "ERROR: [lsm-privileged-helper] $*" >&2
}

# 1. Verificação obrigatória de privilégios de root
if [[ "$(id -u)" -ne 0 ]]; then
    log_err "Este helper deve ser executado exclusivamente como root (via sudo -n)."
    exit 1
fi

if [[ $# -lt 1 ]]; then
    log_err "Uso: $0 <service|journal> [argumentos...]"
    exit 1
fi

SUBCOMMAND="$1"
shift

TIMEOUT_BIN="/usr/bin/timeout"
[[ ! -x "$TIMEOUT_BIN" ]] && TIMEOUT_BIN="/bin/timeout"

case "$SUBCOMMAND" in
    service)
        if [[ $# -ne 2 ]]; then
            log_err "Uso: $0 service <start|stop|restart|reload|status> <service_name.service>"
            exit 1
        fi
        ACTION="$1"
        SERVICE="$2"

        case "$ACTION" in
            start|stop|restart|reload|status)
                ;;
            *)
                log_err "Ação de serviço inválida: '$ACTION'. Permitidas: start, stop, restart, reload, status."
                exit 1
                ;;
        esac

        if [[ ! "$SERVICE" =~ ^[a-zA-Z0-9_@.-]+\.service$ ]]; then
            log_err "Nome de serviço inválido: '$SERVICE'. Padrão obrigatório: ^[a-zA-Z0-9_@.-]+\.service$"
            exit 1
        fi

        SYSTEMCTL_BIN="/bin/systemctl"
        [[ ! -x "$SYSTEMCTL_BIN" ]] && SYSTEMCTL_BIN="/usr/bin/systemctl"
        if [[ ! -x "$SYSTEMCTL_BIN" ]]; then
            log_err "systemctl não encontrado."
            exit 1
        fi

        if [[ -x "$TIMEOUT_BIN" ]]; then
            exec "$TIMEOUT_BIN" 15s "$SYSTEMCTL_BIN" "$ACTION" "$SERVICE"
        else
            exec "$SYSTEMCTL_BIN" "$ACTION" "$SERVICE"
        fi
        ;;

    journal)
        if [[ $# -ne 2 ]]; then
            log_err "Uso: $0 journal <service_name.service> <lines>"
            exit 1
        fi
        SERVICE="$1"
        LINES="$2"

        if [[ ! "$SERVICE" =~ ^[a-zA-Z0-9_@.-]+\.service$ ]]; then
            log_err "Nome de serviço inválido: '$SERVICE'"
            exit 1
        fi

        if [[ ! "$LINES" =~ ^[0-9]+$ ]] || [[ "$LINES" -le 0 ]] || [[ "$LINES" -gt 500 ]]; then
            log_err "Quantidade de linhas inválida: '$LINES' (deve ser entre 1 e 500)"
            exit 1
        fi

        JOURNAL_BIN="/bin/journalctl"
        [[ ! -x "$JOURNAL_BIN" ]] && JOURNAL_BIN="/usr/bin/journalctl"
        if [[ ! -x "$JOURNAL_BIN" ]]; then
            log_err "journalctl não encontrado."
            exit 1
        fi

        if [[ -x "$TIMEOUT_BIN" ]]; then
            exec "$TIMEOUT_BIN" 10s "$JOURNAL_BIN" -u "$SERVICE" -n "$LINES" --no-pager
        else
            exec "$JOURNAL_BIN" -u "$SERVICE" -n "$LINES" --no-pager
        fi
        ;;

    kill)
        log_err "Subcomando 'kill' desabilitado por política de segurança (menor privilégio estrito - Opção A). Operações privilegiadas de término entre contas exigem autorização criptográfica assimétrica independente e pidfd no helper."
        exit 1
        ;;

    *)
        log_err "Subcomando desconhecido ou não autorizado: '$SUBCOMMAND'. Permitidos: service, journal."
        exit 1
        ;;
esac
