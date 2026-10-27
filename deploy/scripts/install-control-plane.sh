#!/usr/bin/env bash
# Linux Server Manager - Control Plane Installer
# Target: Ubuntu Server 24.04 LTS (amd64 / arm64)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/os-detection.sh"
source "${LIB_DIR}/dependencies.sh"
source "${LIB_DIR}/validation.sh"
source "${LIB_DIR}/common.sh"

DOMAIN="${LSM_DOMAIN:-}"
DRY_RUN="false"
SKIP_CADDY="false"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --domain)
            DOMAIN="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --no-caddy)
            SKIP_CADDY="true"
            shift
            ;;
        *)
            log_warn "Parâmetro desconhecido: $1"
            shift
            ;;
    esac
done

log_step "Iniciando instalação oficial do LSM Control Plane..."

# 1. Validação de Sistema Operacional e Arquitetura
detect_os_and_arch "${DRY_RUN}"
check_disk_and_memory

# 2. Dependências
install_dependencies "control-plane" "${DRY_RUN}"

if [[ "${SKIP_CADDY}" != "true" ]]; then
    install_caddy "${DRY_RUN}"
fi

# 3. Usuário e Diretórios Protegidos (Idempotente)
ensure_system_user "lsm-server" "${DRY_RUN}"
ensure_directory "/var/lib/lsm-control-plane" "lsm-server:lsm-server" "750" "${DRY_RUN}"
ensure_directory "/var/www/lsm-web/dist" "www-data:www-data" "755" "${DRY_RUN}"

# 4. Binário do Control Plane
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BIN_DEST="/usr/local/bin/lsm-control-plane"
PRECOMPILED_BIN="${REPO_ROOT}/services/control-plane/lsm-control-plane"

installed_from_precompiled=false
if [[ -f "${PRECOMPILED_BIN}" ]]; then
    if verify_precompiled_binary "${PRECOMPILED_BIN}" "lsm-control-plane" "linux" "${ARCH:-amd64}"; then
        log_info "Instalando binário pré-compilado oficial e verificado a partir de services/control-plane..."
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
    if [[ -d "${REPO_ROOT}/services/control-plane" ]]; then
        if validate_build_toolchain; then
            log_info "Compilando binário do Control Plane com Go local verificado (GOTOOLCHAIN=local)..."
            if [[ "${DRY_RUN}" == "false" ]]; then
                (cd "${REPO_ROOT}/services/control-plane" && GOTOOLCHAIN=local CGO_ENABLED=0 go build -ldflags="-s -w" -o "${BIN_DEST}" ./cmd/server)
                chmod 755 "${BIN_DEST}"
            fi
        else
            log_error "Instalação cancelada: Não há binário pré-compilado verificado e a toolchain Go local não atende à política de produção do LSM (requer Go 1.26.10+ / 1.27.3+)."
            log_error "Solução de Produção: Obtenha o pacote oficial de release do CI contendo os binários verificados e o manifesto SHA256."
            exit 1
        fi
    elif [[ ! -f "${BIN_DEST}" ]]; then
        log_error "Binário lsm-control-plane não encontrado em ${BIN_DEST}. Forneça o executável pré-compilado verificado pelo CI ou instale Go suportado."
        exit 1
    fi
fi

# 5. Frontend Web (SPA)
if [[ -d "${REPO_ROOT}/apps/web/dist" ]]; then
    log_info "Implantando arquivos do frontend estático em /var/www/lsm-web/dist..."
    if [[ "${DRY_RUN}" == "false" ]]; then
        cp -r "${REPO_ROOT}/apps/web/dist/"* /var/www/lsm-web/dist/
        chown -R www-data:www-data /var/www/lsm-web/dist
    fi
fi

# 6. Unidade systemd
SERVICE_SRC="${REPO_ROOT}/deploy/systemd/lsm-control-plane.service"
if [[ -f "${SERVICE_SRC}" ]]; then
    deploy_systemd_unit "lsm-control-plane.service" "${SERVICE_SRC}" "${DRY_RUN}"
    enable_and_start_service "lsm-control-plane.service" "${DRY_RUN}"
fi

# 7. Configuração Caddy (Proxy Reverso)
if [[ "${SKIP_CADDY}" != "true" && -n "${DOMAIN}" && -f "${REPO_ROOT}/deploy/caddy/Caddyfile" ]]; then
    log_step "Configurando proxy reverso Caddy para o domínio: ${DOMAIN}..."
    if [[ "${DRY_RUN}" == "false" ]]; then
        sed "s/lsm.example.com/${DOMAIN}/g" "${REPO_ROOT}/deploy/caddy/Caddyfile" > /etc/caddy/Caddyfile
        if command -v systemctl &>/dev/null && systemctl list-unit-files | grep -q caddy; then
            systemctl reload caddy || systemctl restart caddy
            log_success "Caddy configurado e recarregado com certificado TLS automático."
        fi
    fi
fi

# 8. Verificação de Saúde
if [[ "${DRY_RUN}" == "false" ]]; then
    log_step "Executando validação de integridade HTTP do Control Plane..."
    sleep 2
    if curl -s -f http://127.0.0.1:8080/api/v1/health &>/dev/null; then
        log_success "API do Control Plane respondeu com sucesso em http://127.0.0.1:8080/api/v1/health!"
    else
        log_warn "A API local ainda não respondeu ou inicializou em outra porta. Verifique: journalctl -u lsm-control-plane -n 20"
    fi
fi

log_success "Instalação do Linux Server Manager Control Plane concluída com sucesso!"
