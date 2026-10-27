#!/usr/bin/env bash
# ==============================================================================
# Linux Server Manager (LSM) - Master Automated Production Installer
# Suporte: Ubuntu Server 24.04 LTS (amd64 / arm64)
# Repositório: https://github.com/wallacex19/linux-server-manage
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

source "${LIB_DIR}/logging.sh"

ROLE=""
MODE="install"
PANEL_URL=""
PAIRING_TOKEN=""
DOMAIN=""
PURGE="false"
DRY_RUN="false"

show_help() {
    cat <<EOF
Uso: $0 [OPÇÕES]

Opções Principais:
  --role <control-plane|agent>   Define o papel deste servidor no cluster LSM
  --mode <MODO>                  Ação a executar (padrão: install)
                                   install    : Instalação completa de produção
                                   upgrade    : Atualiza binários e frontend preservando dados
                                   uninstall  : Desinstalação segura de serviços e binários
                                   check      : Diagnóstico de integridade e serviços
                                   dry-run    : Simula as ações sem modificar o sistema

Opções do Control Plane:
  --domain <DOMÍNIO>             Nome de domínio FQDN para emissão de TLS automático no Caddy

Opções do Server Agent:
  --panel-url <URL>              URL do Control Plane (ex: https://painel.exemplo.com)
  --token <PAIRING_TOKEN>        Token de pareamento de uso único gerado no painel central

Opções de Desinstalação:
  --purge                        Remove dados em /var/lib/lsm-* (CUIDADO: destrutivo)
  --keep-data                    Preserva dados de configuração e banco (padrão)

Geral:
  --dry-run                      Modo de simulação (não faz alterações no sistema)
  --help, -h                     Exibe esta ajuda

Exemplos:
  1. Instalar o Painel Central (Control Plane) com domínio:
     sudo $0 --role control-plane --domain lsm.exemplo.com

  2. Instalar o Agente em um servidor gerenciado e parear automaticamente:
     sudo $0 --role agent --panel-url https://lsm.exemplo.com --token 8fbc12a4...

  3. Diagnóstico completo da instalação local:
     sudo $0 --mode check

  4. Atualizar binários preservando dados existentes:
     sudo $0 --role control-plane --mode upgrade

  5. Simular instalação do agente:
     sudo $0 --role agent --panel-url https://lsm.exemplo.com --dry-run
EOF
}

# Processamento de flags da linha de comando
while [[ $# -gt 0 ]]; do
    case "$1" in
        --role)
            ROLE="$2"
            shift 2
            ;;
        --mode)
            MODE="$2"
            shift 2
            ;;
        --panel-url|--panel)
            PANEL_URL="$2"
            shift 2
            ;;
        --token)
            PAIRING_TOKEN="$2"
            shift 2
            ;;
        --domain)
            DOMAIN="$2"
            shift 2
            ;;
        --purge)
            PURGE="true"
            shift
            ;;
        --keep-data)
            PURGE="false"
            shift
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        *)
            log_error "Opção desconhecida: $1. Use --help para instruções de uso."
            exit 1
            ;;
    esac
done

# Se executado sem argumentos em terminal interativo, exibe menu
if [[ -z "${ROLE}" && "${MODE}" == "install" && -t 0 ]]; then
    echo -e "${BOLD}${BLUE}==========================================================${NC}"
    echo -e "${BOLD}${BLUE}       Linux Server Manager - Instalador Automatizado     ${NC}"
    echo -e "${BOLD}${BLUE}==========================================================${NC}"
    echo "Selecione o papel deste servidor no LSM:"
    echo "  1) Control Plane (Painel Central + API + Frontend Web)"
    echo "  2) Server Agent (Nó gerenciado para coleta e operações remotas)"
    echo "  3) Diagnóstico de instalação local (check)"
    echo "  4) Sair"
    read -rp "Opção [1-4]: " OPTION

    case "${OPTION}" in
        1)
            ROLE="control-plane"
            read -rp "Informe o domínio para Caddy TLS (deixe vazio se não usar proxy): " DOMAIN
            ;;
        2)
            ROLE="agent"
            read -rp "Informe a URL pública do Control Plane (ex: https://painel.exemplo.com): " PANEL_URL
            read -rp "Informe o Pairing Token gerado no painel central: " PAIRING_TOKEN
            ;;
        3)
            MODE="check"
            ;;
        *)
            echo "Instalação cancelada pelo usuário."
            exit 0
            ;;
    esac
fi

# Roteamento de acordo com o modo
if [[ "${MODE}" == "check" ]]; then
    bash "${SCRIPT_DIR}/check-installation.sh"
    exit 0
fi

if [[ "${MODE}" == "dry-run" ]]; then
    DRY_RUN="true"
    MODE="install"
fi

if [[ -z "${ROLE}" ]]; then
    log_error "O papel do servidor deve ser especificado via --role <control-plane|agent>."
    show_help
    exit 1
fi

case "${MODE}" in
    install)
        if [[ "${ROLE}" == "control-plane" ]]; then
            ARGS=()
            [[ -n "${DOMAIN}" ]] && ARGS+=(--domain "${DOMAIN}")
            [[ "${DRY_RUN}" == "true" ]] && ARGS+=(--dry-run)
            bash "${SCRIPT_DIR}/install-control-plane.sh" "${ARGS[@]}"
        elif [[ "${ROLE}" == "agent" ]]; then
            ARGS=(--panel-url "${PANEL_URL}")
            [[ -n "${PAIRING_TOKEN}" ]] && ARGS+=(--token "${PAIRING_TOKEN}")
            [[ "${DRY_RUN}" == "true" ]] && ARGS+=(--dry-run)
            bash "${SCRIPT_DIR}/install-agent.sh" "${ARGS[@]}"
        else
            log_error "Papel desconhecido: ${ROLE}. Use 'control-plane' ou 'agent'."
            exit 1
        fi
        ;;
    upgrade)
        ARGS=(--role "${ROLE}")
        [[ "${DRY_RUN}" == "true" ]] && ARGS+=(--dry-run)
        bash "${SCRIPT_DIR}/upgrade.sh" "${ARGS[@]}"
        ;;
    uninstall)
        ARGS=(--role "${ROLE}")
        [[ "${PURGE}" == "true" ]] && ARGS+=(--purge)
        [[ "${DRY_RUN}" == "true" ]] && ARGS+=(--dry-run)
        bash "${SCRIPT_DIR}/uninstall.sh" "${ARGS[@]}"
        ;;
    *)
        log_error "Modo desconhecido: ${MODE}. Modos válidos: install, upgrade, uninstall, check, dry-run."
        exit 1
        ;;
esac
