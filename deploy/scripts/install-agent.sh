#!/usr/bin/env bash
# Linux Server Manager - Server Agent Installer
# Target: Ubuntu Server 24.04 LTS (amd64 / arm64)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/os-detection.sh"
source "${LIB_DIR}/dependencies.sh"
source "${LIB_DIR}/validation.sh"
source "${LIB_DIR}/common.sh"

PANEL_URL="${LSM_PANEL_URL:-}"
PAIRING_TOKEN="${LSM_PAIRING_TOKEN:-}"
DRY_RUN="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --panel-url|--panel)
            PANEL_URL="$2"
            shift 2
            ;;
        --token)
            PAIRING_TOKEN="$2"
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

if [[ -z "${PANEL_URL}" ]]; then
    log_error "Parâmetro obrigatório ausente: --panel-url (ex: https://painel.exemplo.com)"
    exit 1
fi

log_step "Iniciando instalação oficial do LSM Server Agent..."
log_info "URL do Painel Central: ${PANEL_URL}"

# 1. Validação de Sistema Operacional e Arquitetura
detect_os_and_arch "${DRY_RUN}"
check_disk_and_memory

# 2. Dependências
install_dependencies "agent" "${DRY_RUN}"

# 3. Usuário e Diretórios Protegidos (Idempotente)
ensure_system_user "lsm-agent" "${DRY_RUN}"
ensure_directory "/var/lib/lsm-agent" "lsm-agent:lsm-agent" "750" "${DRY_RUN}"
ensure_directory "/var/log/lsm-agent" "lsm-agent:lsm-agent" "750" "${DRY_RUN}"
ensure_directory "/usr/local/libexec" "root:root" "755" "${DRY_RUN}"

# 4. Instalação do Privileged Helper e Sudoers Restrito
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HELPER_SRC="${SCRIPT_DIR}/lsm-privileged-helper.sh"
HELPER_DEST="/usr/local/libexec/lsm-privileged-helper"

log_step "Configurando Privileged Helper para menor privilégio..."
if [[ "${DRY_RUN}" == "false" ]]; then
    if [[ -f "${HELPER_SRC}" ]]; then
        cp "${HELPER_SRC}" "${HELPER_DEST}"
        chown root:lsm-agent "${HELPER_DEST}"
        chmod 750 "${HELPER_DEST}"
        log_success "Privileged Helper instalado em ${HELPER_DEST} (750 root:lsm-agent)."
    fi

    SUDOERS_FILE="/etc/sudoers.d/lsm-agent"
    cat <<EOF > "${SUDOERS_FILE}"
# Linux Server Manager - Menor privilégio estrito para o Server Agent
# Permite ao usuário de serviço lsm-agent invocar exclusivamente o helper para serviços systemd e logs (Opção A)
lsm-agent ALL=(root) NOPASSWD: /usr/local/libexec/lsm-privileged-helper service *, /usr/local/libexec/lsm-privileged-helper journal *
EOF
    chmod 0440 "${SUDOERS_FILE}"
    if command -v visudo &>/dev/null; then
        if ! visudo -c -f "${SUDOERS_FILE}" >/dev/null 2>&1; then
            log_error "Erro de validação de sintaxe no arquivo sudoers gerado (${SUDOERS_FILE}). Abortando instalação."
            rm -f "${SUDOERS_FILE}"
            exit 1
        fi
    fi
    log_success "Regra sudoers restrita configurada em ${SUDOERS_FILE}."
else
    log_info "[DRY-RUN] Simulação: Privileged Helper e regra sudoers restrita não foram modificados."
fi

# 5. Binário do Server Agent
BIN_DEST="/usr/local/bin/lsm-agent"
PRECOMPILED_BIN="${REPO_ROOT}/services/server-agent/lsm-agent"

installed_from_precompiled=false
if [[ -f "${PRECOMPILED_BIN}" ]]; then
    if verify_precompiled_binary "${PRECOMPILED_BIN}" "lsm-agent" "linux" "${ARCH:-amd64}"; then
        log_info "Instalando binário pré-compilado oficial e verificado a partir de services/server-agent..."
        if [[ "${DRY_RUN}" == "false" ]]; then
            cp "${PRECOMPILED_BIN}" "${BIN_DEST}"
            chmod 755 "${BIN_DEST}"
        fi
        installed_from_precompiled=true
    else
        log_warn "O binário pré-compilado existente em ${PRECOMPILED_BIN} não atende aos requisitos de integridade/arquitetura/toolchain de produção e NÃO será utilizado."
    fi
fi

if [[ "${installed_from_precompiled}" != "true" ]]; then
    if [[ -d "${REPO_ROOT}/services/server-agent" ]]; then
        if validate_build_toolchain; then
            log_info "Compilando binário do Server Agent com Go local verificado (GOTOOLCHAIN=local)..."
            if [[ "${DRY_RUN}" == "false" ]]; then
                (cd "${REPO_ROOT}/services/server-agent" && GOTOOLCHAIN=local CGO_ENABLED=0 go build -ldflags="-s -w" -o "${BIN_DEST}" ./cmd/agent)
                chmod 755 "${BIN_DEST}"
            fi
        else
            log_error "Instalação cancelada: Não há binário pré-compilado verificado e a toolchain Go local não atende à política de produção do LSM (requer Go 1.26.10+ / 1.27.3+)."
            log_error "Solução de Produção: Obtenha o pacote oficial de release do CI contendo os binários verificados e o manifesto SHA256."
            exit 1
        fi
    elif [[ ! -f "${BIN_DEST}" ]]; then
        log_error "Binário lsm-agent não encontrado em ${BIN_DEST}. Forneça o executável pré-compilado verificado pelo CI ou instale Go suportado."
        exit 1
    fi
fi

# 6. Identidade Criptográfica (Ed25519)
KEY_FILE="/var/lib/lsm-agent/identity.key"
CONFIG_FILE="/var/lib/lsm-agent/agent_config.json"

if [[ "${DRY_RUN}" == "false" && -x "${BIN_DEST}" ]]; then
    log_step "Verificando/inicializando identidade criptográfica Ed25519..."
    PUB_KEY=$("${BIN_DEST}" --export-pubkey --identity-path "${KEY_FILE}" 2>/dev/null || echo "")
    if [[ -f "${KEY_FILE}" ]]; then
        chown lsm-agent:lsm-agent "${KEY_FILE}" 2>/dev/null || true
        chmod 600 "${KEY_FILE}"
    fi
    log_success "Identidade Ed25519 validada (Chave pública: ${PUB_KEY:0:16}...)."

    # 7. Pareamento com o Painel Central (se pairing token fornecido e ainda não pareado)
    if [[ -n "${PAIRING_TOKEN}" && ! -f "${CONFIG_FILE}" ]]; then
        log_step "Executando pareamento automatizado com o Painel Central..."
        SIG=$("${BIN_DEST}" --sign-token "${PAIRING_TOKEN}" --identity-path "${KEY_FILE}" 2>/dev/null || echo "")

        PAIR_PAYLOAD=$(cat <<EOF
{
  "pairing_token": "${PAIRING_TOKEN}",
  "public_key": "${PUB_KEY}",
  "signature": "${SIG}",
  "hostname": "$(hostname)",
  "ip_address": "$(hostname -I 2>/dev/null | awk '{print $1}' || echo '127.0.0.1')",
  "os_info": "${OS_NAME} ${OS_VERSION_ID} (${SYSTEM_ARCH})",
  "agent_version": "1.2.2"
}
EOF
)

        PAIR_RESP=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "${PAIR_PAYLOAD}" \
            "${PANEL_URL}/api/v1/agent/pair" || echo "{}")

        NODE_ID=$(echo "${PAIR_RESP}" | grep -o '"node_id":"[^"]*' | cut -d'"' -f4 || echo "")
        AGENT_TOKEN=$(echo "${PAIR_RESP}" | grep -o '"agent_token":"[^"]*' | cut -d'"' -f4 || echo "")

        if [[ -n "${NODE_ID}" && -n "${AGENT_TOKEN}" ]]; then
            cat <<EOF > "${CONFIG_FILE}"
{
  "node_id": "${NODE_ID}",
  "agent_token": "${AGENT_TOKEN}",
  "panel_url": "${PANEL_URL}",
  "hostname": "$(hostname)",
  "identity_path": "${KEY_FILE}",
  "poll_interval_seconds": 3,
  "privileged_helper_path": "${HELPER_DEST}"
}
EOF
            chown lsm-agent:lsm-agent "${CONFIG_FILE}"
            chmod 600 "${CONFIG_FILE}"
            log_success "Pareamento bem-sucedido! Node ID: ${NODE_ID}"
        else
            log_warn "Pareamento via API retornou: ${PAIR_RESP}. Configure ${CONFIG_FILE} manualmente se necessário."
        fi
    fi
fi

# 8. Unidade systemd
SERVICE_SRC="${REPO_ROOT}/deploy/systemd/lsm-agent.service"
if [[ -f "${SERVICE_SRC}" ]]; then
    deploy_systemd_unit "lsm-agent.service" "${SERVICE_SRC}" "${DRY_RUN}"
    enable_and_start_service "lsm-agent.service" "${DRY_RUN}"
fi

log_success "Instalação do Linux Server Manager Agent concluída com sucesso!"
