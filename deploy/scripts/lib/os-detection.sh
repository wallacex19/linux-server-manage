#!/usr/bin/env bash
# Linux Server Manager - OS & Architecture Detection Library

detect_os_and_arch() {
    local dry_run="${1:-false}"

    # 1. Verifica se está sendo executado como root (EUID 0)
    if [[ "${EUID}" -ne 0 ]]; then
        if [[ "${dry_run}" == "true" ]]; then
            log_dry "Execução não-root detectada; prosseguindo em modo de simulação dry-run."
        else
            log_error "Este instalador requer privilégios de superusuário (root ou sudo)."
            return 1
        fi
    fi

    # 2. Verifica se o sistema possui systemd
    if ! command -v systemctl &>/dev/null; then
        if [[ "${dry_run}" == "true" ]]; then
            log_dry "systemctl não encontrado no ambiente atual; ignorado em modo dry-run."
        else
            log_error "O sistema operacional não possui 'systemd'. O Linux Server Manager exige systemd para gerenciamento de serviços."
            return 1
        fi
    fi

    # 3. Detecta distribuição Linux via /etc/os-release
    if [[ ! -f /etc/os-release ]]; then
        if [[ "${dry_run}" == "true" ]]; then
            log_dry "/etc/os-release não encontrado; assumindo simulação de Ubuntu 24.04 LTS (amd64)."
            OS_ID="ubuntu"
            OS_VERSION_ID="24.04"
            OS_NAME="Ubuntu"
            SYSTEM_ARCH="amd64"
            return 0
        else
            log_error "Arquivo /etc/os-release não encontrado. Distribuição Linux não identificada."
            return 1
        fi
    fi

    # shellcheck source=/dev/null
    source /etc/os-release
    OS_ID="${ID:-unknown}"
    OS_VERSION_ID="${VERSION_ID:-unknown}"
    OS_NAME="${NAME:-Linux}"

    log_info "Sistema Operacional detectado: ${OS_NAME} (${OS_ID} ${OS_VERSION_ID})"

    # Verificação de compatibilidade (Ubuntu 24.04 LTS preferencial, ou Debian 12+)
    if [[ "${OS_ID}" == "ubuntu" ]]; then
        MAJOR_VER=$(echo "${OS_VERSION_ID}" | cut -d. -f1)
        if [[ "${MAJOR_VER}" -lt 22 ]]; then
            log_warn "Versão do Ubuntu (${OS_VERSION_ID}) é anterior à recomendada (24.04 LTS). O funcionamento pode ser afetado."
        elif [[ "${OS_VERSION_ID}" == "24.04" ]]; then
            log_success "Ubuntu 24.04 LTS detectado (Distribuição de referência oficial)."
        fi
    elif [[ "${OS_ID}" == "debian" ]]; then
        log_info "Distribuição base Debian detectada. Compatível com a stack do LSM."
    else
        log_warn "Distribuição '${OS_ID}' não é Ubuntu Server. Tentando continuar em modo compatível..."
    fi

    # 4. Detecta arquitetura do processador
    RAW_ARCH=$(uname -m)
    case "${RAW_ARCH}" in
        x86_64)
            SYSTEM_ARCH="amd64"
            ;;
        aarch64|arm64)
            SYSTEM_ARCH="arm64"
            ;;
        *)
            log_error "Arquitetura não suportada: ${RAW_ARCH}. O LSM suporta apenas amd64 (x86_64) e arm64."
            return 1
            ;;
    esac

    log_info "Arquitetura de processador detectada: ${SYSTEM_ARCH} (${RAW_ARCH})"
    return 0
}
