#!/usr/bin/env bash
# ==============================================================================
# Linux Server Manager - Bootstrap Host Setup
# Target: Ubuntu Server 24.04 LTS
# Prepara o ambiente e executa o instalador do LSM com versão fixada e dry-run real
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
PANEL_URL=""
PAIRING_TOKEN=""
NODE_ID=""
DOMAIN=""
ROLE="agent"
DRY_RUN="false"
LSM_VERSION="${LSM_VERSION:-v1.2.2}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --panel|--panel-url)
            PANEL_URL="$2"
            shift 2
            ;;
        --token)
            PAIRING_TOKEN="$2"
            shift 2
            ;;
        --node-id)
            NODE_ID="$2"
            shift 2
            ;;
        --domain)
            DOMAIN="$2"
            shift 2
            ;;
        --role)
            ROLE="$2"
            shift 2
            ;;
        --version)
            LSM_VERSION="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

if [[ "${ROLE}" != "control-plane" && "${ROLE}" != "agent" ]]; then
    echo "ERRO: Papel inválido '${ROLE}'. Papéis suportados individualmente: 'control-plane' ou 'agent'." >&2
    exit 1
fi

echo "=== Linux Server Manager - Bootstrap Setup (Versão: ${LSM_VERSION}) ==="

# 1. Tratamento estrito do modo Dry-Run antes de qualquer alteração de sistema
if [[ "${DRY_RUN}" == "true" ]]; then
    echo "[DRY-RUN] Simulação de bootstrap ativada. Nenhuma modificação no sistema será realizada."
    echo "[DRY-RUN] Papel selecionado: ${ROLE}"
    echo "[DRY-RUN] Versão alvo fixada: ${LSM_VERSION}"
    [[ -n "${PANEL_URL}" ]] && echo "[DRY-RUN] URL do Painel: ${PANEL_URL}"
    [[ -n "${NODE_ID}" ]] && echo "[DRY-RUN] ID do Nó: ${NODE_ID}"
    echo "[DRY-RUN] Usuários a provisionar: lsm-server (control-plane) e/ou lsm-agent (agent)"
    echo "[DRY-RUN] Diretórios de estado: /var/lib/lsm-control-plane, /var/lib/lsm-agent (750)"
    echo "[DRY-RUN] Concluindo simulação com sucesso."
    exit 0
fi

# Diretório temporário de bootstrap limpo ao término
BOOTSTRAP_DIR=""
cleanup() {
    if [[ -n "${BOOTSTRAP_DIR}" && -d "${BOOTSTRAP_DIR}" ]]; then
        rm -rf "${BOOTSTRAP_DIR}"
    fi
}
trap cleanup EXIT

# 2. Criação estrita do usuário de sistema para o papel solicitado
if [[ "$ROLE" == "control-plane" ]]; then
    if ! id "lsm-server" &>/dev/null; then
        echo "Criando usuário de sistema lsm-server..."
        useradd --system --no-create-home --shell /usr/sbin/nologin lsm-server
    fi
    mkdir -p /var/lib/lsm-control-plane
    chown -R lsm-server:lsm-server /var/lib/lsm-control-plane
    chmod 750 /var/lib/lsm-control-plane
fi

if [[ "$ROLE" == "agent" ]]; then
    if ! id "lsm-agent" &>/dev/null; then
        echo "Criando usuário de sistema lsm-agent..."
        useradd --system --no-create-home --shell /usr/sbin/nologin lsm-agent
    fi
    mkdir -p /var/lib/lsm-agent /var/log/lsm-agent
    chown -R lsm-agent:lsm-agent /var/lib/lsm-agent /var/log/lsm-agent
    chmod 750 /var/lib/lsm-agent
fi

echo "=== Usuários de sistema e diretórios base configurados ==="

# 3. Localização do instalador local ou download com versão estritamente fixada
INSTALLER_CMD=""

if [[ -n "${SCRIPT_DIR}" && -f "${SCRIPT_DIR}/install.sh" ]]; then
    INSTALLER_CMD="${SCRIPT_DIR}/install.sh"
elif [[ -n "${SCRIPT_DIR}" && -f "${SCRIPT_DIR}/install-agent.sh" && "$ROLE" == "agent" ]]; then
    INSTALLER_CMD="${SCRIPT_DIR}/install-agent.sh"
fi

if [[ -z "${INSTALLER_CMD}" ]]; then
    echo "Instalador local não encontrado. Obtendo pacote da versão fixada '${LSM_VERSION}'..."
    BOOTSTRAP_DIR=$(mktemp -d /tmp/lsm-bootstrap-XXXXXX)

    CLONE_SUCCESS="false"
    if command -v git &>/dev/null; then
        if git clone --depth 1 --branch "${LSM_VERSION}" https://github.com/wallacex19/linux-server-manage.git "${BOOTSTRAP_DIR}" 2>/dev/null; then
            CLONE_SUCCESS="true"
        fi
    fi

    if [[ "${CLONE_SUCCESS}" == "false" ]] && command -v curl &>/dev/null; then
        TAR_URL="https://github.com/wallacex19/linux-server-manage/archive/refs/tags/${LSM_VERSION}.tar.gz"
        if ! curl -sSLf "${TAR_URL}" | tar -xz -C "${BOOTSTRAP_DIR}" --strip-components=1 2>/dev/null; then
            # Tenta branch com nome idêntico
            BRANCH_URL="https://github.com/wallacex19/linux-server-manage/archive/refs/heads/${LSM_VERSION}.tar.gz"
            if curl -sSLf "${BRANCH_URL}" | tar -xz -C "${BOOTSTRAP_DIR}" --strip-components=1 2>/dev/null; then
                CLONE_SUCCESS="true"
            fi
        else
            CLONE_SUCCESS="true"
        fi
    fi

    if [[ "${CLONE_SUCCESS}" == "false" ]]; then
        echo "ERRO CRÍTICO: Não foi possível obter o instalador para a versão fixada '${LSM_VERSION}'. Instalação abortada por segurança." >&2
        exit 1
    fi

    # Validação de Autenticidade e Integridade do Bootstrap Inicial (Root-of-Trust)
    # Impede execução de código mutável como root sem conferência out-of-band
    if [[ -n "${LSM_BOOTSTRAP_SHA256:-}" ]]; then
        echo "Validando hash SHA-256 do instalador contra autoridade out-of-band..."
        actual_inst_hash=$(sha256sum "${BOOTSTRAP_DIR}/deploy/scripts/install.sh" 2>/dev/null | awk '{print $1}' || echo "")
        if [[ "${actual_inst_hash}" != "${LSM_BOOTSTRAP_SHA256}" ]]; then
            echo "ERRO CRÍTICO DE SEGURANÇA: O hash do instalador obtido (${actual_inst_hash}) não confere com o hash esperado (${LSM_BOOTSTRAP_SHA256})!" >&2
            exit 1
        fi
        echo "Integridade do bootstrap confirmada com sucesso."
    fi

    if [[ -d "${BOOTSTRAP_DIR}/.git" ]]; then
        if git -C "${BOOTSTRAP_DIR}" tag -v "${LSM_VERSION}" &>/dev/null; then
            echo "Tag oficial '${LSM_VERSION}' verificada com sucesso via assinatura criptográfica do Git."
        elif [[ "${LSM_REQUIRE_SIGNED_TAG:-false}" == "true" ]]; then
            echo "ERRO CRÍTICO DE SEGURANÇA: A tag '${LSM_VERSION}' não possui assinatura digital válida. Abortando execução por política de segurança." >&2
            exit 1
        fi
    fi

    INSTALLER_CMD="${BOOTSTRAP_DIR}/deploy/scripts/install.sh"
fi

if [[ ! -f "${INSTALLER_CMD}" ]]; then
    echo "ERRO: Script de instalação não encontrado: ${INSTALLER_CMD}" >&2
    exit 1
fi

chmod +x "${INSTALLER_CMD}"

# 4. Invoca o instalador com os parâmetros fornecidos
INSTALL_ARGS=(--role "${ROLE}")
if [[ -n "${DOMAIN}" && "${ROLE}" == "control-plane" ]]; then
    INSTALL_ARGS+=(--domain "${DOMAIN}")
fi
if [[ -n "${PANEL_URL}" && "${ROLE}" == "agent" ]]; then
    INSTALL_ARGS+=(--panel-url "${PANEL_URL}")
fi
if [[ -n "${PAIRING_TOKEN}" && "${ROLE}" == "agent" ]]; then
    INSTALL_ARGS+=(--token "${PAIRING_TOKEN}")
fi
if [[ -n "${NODE_ID}" && "${ROLE}" == "agent" ]]; then
    INSTALL_ARGS+=(--node-id "${NODE_ID}")
fi
if [[ "${DRY_RUN}" == "true" ]]; then
    INSTALL_ARGS+=(--dry-run)
fi

echo "Executando: ${INSTALLER_CMD} ${INSTALL_ARGS[*]}"
exec bash "${INSTALLER_CMD}" "${INSTALL_ARGS[@]}"
