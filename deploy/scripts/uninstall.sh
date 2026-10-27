#!/usr/bin/env bash
# Linux Server Manager - Uninstaller Script
# Por padrão, preserva os dados (/var/lib/lsm-*) exceto se explicitamente solicitado com --purge

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/os-detection.sh"

ROLE=""
PURGE_DATA="false"
DRY_RUN="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --role)
            ROLE="$2"
            shift 2
            ;;
        --purge)
            PURGE_DATA="true"
            shift
            ;;
        --keep-data)
            PURGE_DATA="false"
            shift
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        *)
            log_warn "Parâmetro desconhecido: $1"
            shift
            ;;
    esac
done

if [[ -z "${ROLE}" ]]; then
    log_error "Parâmetro obrigatório ausente: --role <control-plane|agent|all>"
    exit 1
fi

log_step "Iniciando desinstalação segura do Linux Server Manager (Role: ${ROLE})..."
detect_os_and_arch "${DRY_RUN}"

uninstall_cp() {
    log_info "Parando e desativando serviço lsm-control-plane..."
    if [[ "${DRY_RUN}" == "false" ]]; then
        systemctl stop lsm-control-plane.service 2>/dev/null || true
        systemctl disable lsm-control-plane.service 2>/dev/null || true
        rm -f /etc/systemd/system/lsm-control-plane.service
        rm -f /usr/local/bin/lsm-control-plane
        systemctl daemon-reload
    else
        log_dry "Removeria serviço e binário do Control Plane"
    fi

    if [[ "${PURGE_DATA}" == "true" ]]; then
        log_warn "REMOVENDO DADOS: /var/lib/lsm-control-plane e /var/www/lsm-web..."
        if [[ "${DRY_RUN}" == "false" ]]; then
            rm -rf /var/lib/lsm-control-plane /var/www/lsm-web
        fi
    else
        log_info "Dados preservados em /var/lib/lsm-control-plane (Use --purge se desejar remover o banco de dados)."
    fi
}

uninstall_ag() {
    log_info "Parando e desativando serviço lsm-agent..."
    if [[ "${DRY_RUN}" == "false" ]]; then
        systemctl stop lsm-agent.service 2>/dev/null || true
        systemctl disable lsm-agent.service 2>/dev/null || true
        rm -f /etc/systemd/system/lsm-agent.service
        rm -f /usr/local/bin/lsm-agent
        rm -f /usr/local/libexec/lsm-privileged-helper
        rm -f /etc/sudoers.d/lsm-agent
        systemctl daemon-reload
    else
        log_dry "Removeria serviço, binário, privileged helper e sudoers do Server Agent"
    fi

    if [[ "${PURGE_DATA}" == "true" ]]; then
        log_warn "REMOVENDO DADOS: /var/lib/lsm-agent e /var/log/lsm-agent..."
        if [[ "${DRY_RUN}" == "false" ]]; then
            rm -rf /var/lib/lsm-agent /var/log/lsm-agent
        fi
    else
        log_info "Identidade e dados preservados em /var/lib/lsm-agent."
    fi
}

case "${ROLE}" in
    control-plane)
        uninstall_cp
        ;;
    agent)
        uninstall_ag
        ;;
    all)
        uninstall_cp
        uninstall_ag
        ;;
    *)
        log_error "Opção de papel desconhecida: ${ROLE}. Use control-plane, agent ou all."
        exit 1
        ;;
esac

log_success "Desinstalação concluída com sucesso."
