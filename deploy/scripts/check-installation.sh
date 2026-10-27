#!/usr/bin/env bash
# Linux Server Manager - Installation Diagnostics & Health Check

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/os-detection.sh"

log_step "=========================================================="
log_step "   Linux Server Manager - Diagnóstico de Instalação      "
log_step "=========================================================="

detect_os_and_arch || true

log_step "1. Status dos Serviços systemd:"
for svc in "lsm-control-plane.service" "lsm-agent.service" "caddy.service"; do
    if systemctl list-unit-files "${svc}" &>/dev/null; then
        STATUS=$(systemctl is-active "${svc}" 2>/dev/null || echo "inactive")
        if [[ "${STATUS}" == "active" ]]; then
            log_success "Serviço ${svc}: ATIVO (active/running)"
        else
            log_warn "Serviço ${svc}: ${STATUS}"
        fi
    else
        log_info "Serviço ${svc}: não instalado neste host"
    fi
done

log_step "2. Auditoria de Permissões e Diretórios:"
check_dir_perms() {
    local dir="$1"
    local expected_owner="$2"
    local expected_mode="$3"

    if [[ -d "${dir}" ]]; then
        local current_owner
        current_owner=$(stat -c '%U:%G' "${dir}" 2>/dev/null || stat -f '%Su:%Sg' "${dir}" 2>/dev/null || echo "unknown")
        local current_mode
        current_mode=$(stat -c '%a' "${dir}" 2>/dev/null || stat -f '%A' "${dir}" 2>/dev/null || echo "unknown")

        if [[ "${current_owner}" == "${expected_owner}" && "${current_mode}" == *"${expected_mode}"* ]]; then
            log_success "Diretório ${dir}: OK (Dono: ${current_owner}, Permissões: ${current_mode})"
        else
            log_warn "Diretório ${dir}: Alerta (Dono: ${current_owner} [esp: ${expected_owner}], Permissões: ${current_mode} [esp: ${expected_mode}])"
        fi
    else
        log_info "Diretório ${dir}: não existe (papel não instalado)"
    fi
}

check_dir_perms "/var/lib/lsm-control-plane" "lsm-server:lsm-server" "750"
check_dir_perms "/var/lib/lsm-agent" "lsm-agent:lsm-agent" "750"
check_dir_perms "/var/log/lsm-agent" "lsm-agent:lsm-agent" "750"

log_step "3. Auditoria de Chaves Criptográficas:"
KEY_FILE="/var/lib/lsm-agent/identity.key"
if [[ -f "${KEY_FILE}" ]]; then
    KEY_PERM=$(stat -c '%a' "${KEY_FILE}" 2>/dev/null || stat -f '%A' "${KEY_FILE}" 2>/dev/null || echo "unknown")
    if [[ "${KEY_PERM}" == "600" || "${KEY_PERM}" == "-rw-------" ]]; then
        log_success "Chave de identidade do agente ${KEY_FILE}: Permissão estrita 600 verificada."
    else
        log_error "Chave de identidade ${KEY_FILE} possui permissões inseguras: ${KEY_PERM}! Deve ser 600."
    fi
else
    log_info "Chave de identidade do agente não encontrada neste host."
fi

log_step "4. Teste de Conectividade e Saúde da API REST:"
if curl -s -f http://127.0.0.1:8080/api/v1/health &>/dev/null; then
    HEALTH_RESP=$(curl -s http://127.0.0.1:8080/api/v1/health)
    log_success "Healthcheck API OK: ${HEALTH_RESP}"
    if curl -s -f http://127.0.0.1:8080/api/v1/version &>/dev/null; then
        VER_RESP=$(curl -s http://127.0.0.1:8080/api/v1/version)
        log_success "Versão da API: ${VER_RESP}"
    fi
else
    log_info "Control Plane local na porta 8080 não respondeu ou não está ativo neste host."
fi

log_step "=========================================================="
log_success "Diagnóstico do Linux Server Manager concluído."
log_step "=========================================================="
