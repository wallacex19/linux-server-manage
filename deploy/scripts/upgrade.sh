#!/usr/bin/env bash
# Linux Server Manager - Idempotent Upgrade Script
# Preserva integralmente bancos de dados SQLite, identidades criptográficas e configurações

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/os-detection.sh"
source "${LIB_DIR}/common.sh"

ROLE=""
DRY_RUN="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --role)
            ROLE="$2"
            shift 2
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
    log_error "Parâmetro obrigatório ausente: --role <control-plane|agent>"
    exit 1
fi

log_step "Iniciando atualização idempotente para: ${ROLE}..."
detect_os_and_arch

REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

if [[ "${ROLE}" == "control-plane" ]]; then
    SERVICE_NAME="lsm-control-plane.service"
    BIN_DEST="/usr/local/bin/lsm-control-plane"

    log_info "Atualizando binário do Control Plane..."
    if [[ -f "${REPO_ROOT}/services/control-plane/lsm-control-plane" && "${DRY_RUN}" == "false" ]]; then
        cp "${REPO_ROOT}/services/control-plane/lsm-control-plane" "${BIN_DEST}"
        chmod 755 "${BIN_DEST}"
    fi

    log_info "Atualizando arquivos estáticos do frontend em /var/www/lsm-web/dist..."
    if [[ -d "${REPO_ROOT}/apps/web/dist" && "${DRY_RUN}" == "false" ]]; then
        cp -r "${REPO_ROOT}/apps/web/dist/"* /var/www/lsm-web/dist/
        chown -R www-data:www-data /var/www/lsm-web/dist
    fi

    log_info "Reiniciando serviço ${SERVICE_NAME}..."
    if [[ "${DRY_RUN}" == "false" ]]; then
        systemctl restart "${SERVICE_NAME}"
        sleep 2
        if systemctl is-active --quiet "${SERVICE_NAME}"; then
            log_success "Control Plane atualizado e em execução ativa."
        fi
    fi

elif [[ "${ROLE}" == "agent" ]]; then
    SERVICE_NAME="lsm-agent.service"
    BIN_DEST="/usr/local/bin/lsm-agent"

    log_info "Atualizando binário do Server Agent..."
    if [[ -f "${REPO_ROOT}/services/server-agent/lsm-agent" && "${DRY_RUN}" == "false" ]]; then
        cp "${REPO_ROOT}/services/server-agent/lsm-agent" "${BIN_DEST}"
        chmod 755 "${BIN_DEST}"
    fi

    log_info "Reiniciando serviço ${SERVICE_NAME}..."
    if [[ "${DRY_RUN}" == "false" ]]; then
        systemctl restart "${SERVICE_NAME}"
        sleep 2
        if systemctl is-active --quiet "${SERVICE_NAME}"; then
            log_success "Server Agent atualizado e em execução ativa."
        fi
    fi
fi

log_success "Atualização do ${ROLE} concluída preservando dados e credenciais!"
