#!/usr/bin/env bash
# Linux Server Manager - Dependencies Management Library

install_dependencies() {
    local role="$1"
    local dry_run="${2:-false}"

    log_step "Verificando dependências do sistema para papel: ${role}..."

    local pkg_manager=""
    if command -v apt-get &>/dev/null; then
        pkg_manager="apt"
    elif command -v dnf &>/dev/null; then
        pkg_manager="dnf"
    elif command -v yum &>/dev/null; then
        pkg_manager="yum"
    else
        log_warn "Gerenciador de pacotes apt/dnf/yum não encontrado. Verifique dependências manualmente."
        return 0
    fi

    local required_pkgs=("curl" "tar" "tmux" "ca-certificates")

    if [[ "${role}" == "control-plane" ]]; then
        required_pkgs+=("sqlite3")
    elif [[ "${role}" == "agent" ]]; then
        required_pkgs+=("iproute2" "procps")
    fi

    local missing_pkgs=()
    for pkg in "${required_pkgs[@]}"; do
        if ! command -v "${pkg}" &>/dev/null; then
            missing_pkgs+=("${pkg}")
        fi
    done

    if [[ ${#missing_pkgs[@]} -eq 0 ]]; then
        log_success "Todas as dependências básicas já estão instaladas."
        return 0
    fi

    log_info "Pacotes ausentes a instalar: ${missing_pkgs[*]}"

    if [[ "${dry_run}" == "true" ]]; then
        log_dry "Executaria atualização de repositórios e instalação dos pacotes: ${missing_pkgs[*]}"
        return 0
    fi

    if [[ "${pkg_manager}" == "apt" ]]; then
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq
        apt-get install -y -qq "${missing_pkgs[@]}"
    elif [[ "${pkg_manager}" == "dnf" || "${pkg_manager}" == "yum" ]]; then
        "${pkg_manager}" install -y -q "${missing_pkgs[@]}"
    fi

    log_success "Dependências do sistema instaladas com sucesso."
}

install_caddy() {
    local dry_run="${1:-false}"
    if command -v caddy &>/dev/null; then
        log_info "Caddy Web Server já está instalado ($(caddy version | head -n1))."
        return 0
    fi

    log_step "Instalando Caddy como proxy reverso oficial..."
    if [[ "${dry_run}" == "true" ]]; then
        log_dry "Executaria instalação oficial do Caddy via repositório APT"
        return 0
    fi

    if command -v apt-get &>/dev/null; then
        apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https curl
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg --yes
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
        apt-get update -qq
        apt-get install -y -qq caddy
        log_success "Caddy instalado com sucesso."
    else
        log_warn "Instalação automática do Caddy suportada nativamente em Debian/Ubuntu. Instale Caddy manualmente para proxy reverso."
    fi
}

validate_build_toolchain() {
    local raw_ver="${1:-}"
    if [[ -z "${raw_ver}" ]]; then
        if ! command -v go &>/dev/null; then
            return 1
        fi
        raw_ver=$(go version | awk '{print $3}' | sed 's/go//')
    else
        # Remove prefixo "go" se presente
        raw_ver="${raw_ver#go}"
    fi

    # Rejeita versões com caracteres não-numéricos (pré-lançamentos, beta, rc, etc.)
    if [[ "${raw_ver}" =~ [^0-9.] ]]; then
        log_error "Versão Go não aprovada: '${raw_ver}' contém sufixos de pré-lançamento ou formato não homologado."
        return 2
    fi

    local major minor patch
    major=$(echo "${raw_ver}" | cut -d. -f1)
    minor=$(echo "${raw_ver}" | cut -d. -f2)
    patch=$(echo "${raw_ver}" | cut -d. -f3)
    patch="${patch:-0}"

    local is_approved=false

    # Matriz oficial estrita de toolchains suportadas para produção:
    # Apenas patches estáveis homologados: Go 1.26.10+ e Go 1.27.3+.
    # Versões anteriores e versões futuras não homologadas (ex: Go >= 1.28) são rejeitadas.
    if [[ "${major}" -eq 1 ]]; then
        if [[ "${minor}" -eq 26 && "${patch}" -ge 10 ]]; then
            is_approved=true
        elif [[ "${minor}" -eq 27 && "${patch}" -ge 3 ]]; then
            is_approved=true
        fi
    fi

    if [[ "${is_approved}" == "true" ]]; then
        log_success "Toolchain Go verificada (${raw_ver}) em conformidade estrita com a política de produção do LSM."
        return 0
    fi

    local env_mode="${LSM_ENV:-production}"
    log_warn "Toolchain Go detectada: ${raw_ver} (fora da política de homologação de produção)."

    # Em modo de produção, a política é estrita: LSM_ALLOW_UNSUPPORTED_COMPILER é rejeitado
    if [[ "${env_mode}" == "production" ]]; then
        if [[ "${LSM_ALLOW_UNSUPPORTED_COMPILER:-false}" == "true" ]]; then
            log_error "VIOLAÇÃO DE POLÍTICA: LSM_ALLOW_UNSUPPORTED_COMPILER=true é terminantemente proibido quando LSM_ENV=production."
        fi
        log_error "Compilação de produção bloqueada: Go ${raw_ver} não é uma versão aprovada pela política (aprovadas: Go 1.26.10+ e Go 1.27.3+)."
        log_error "Versões desatualizadas, sem patches de segurança ou futuras ainda não validadas são rejeitadas."
        return 2
    fi

    # Em ambiente de desenvolvimento ou teste, o bypass com aviso é permitido
    if [[ "${LSM_ALLOW_UNSUPPORTED_COMPILER:-false}" == "true" || "${env_mode}" == "development" || "${env_mode}" == "test" ]]; then
        log_warn "AVISO DE RISCO (LSM_ALLOW_UNSUPPORTED_COMPILER=true): Compilação permitida exclusivamente para ambiente de teste/desenvolvimento (${env_mode})."
        log_warn "Esta versão Go (${raw_ver}) não é homologada para produção."
        return 0
    fi

    log_error "Compilação bloqueada: Go ${raw_ver} não atende aos requisitos mínimos de segurança de toolchain."
    return 2
}
