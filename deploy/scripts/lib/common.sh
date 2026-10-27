#!/usr/bin/env bash
# Linux Server Manager - Common Utilities Library

ensure_system_user() {
    local username="$1"
    local dry_run="${2:-false}"

    if id "${username}" &>/dev/null; then
        log_info "Usuário de sistema '${username}' já existe."
        return 0
    fi

    log_step "Criando usuário de sistema dedicado: ${username}..."
    if [[ "${dry_run}" == "true" ]]; then
        log_dry "Executaria: useradd --system --no-create-home --shell /usr/sbin/nologin ${username}"
        return 0
    fi

    useradd --system --no-create-home --shell /usr/sbin/nologin "${username}"
    log_success "Usuário '${username}' criado com shell restrito (nologin)."
}

ensure_directory() {
    local dir_path="$1"
    local owner="$2"
    local permissions="$3"
    local dry_run="${4:-false}"

    if [[ "${dry_run}" == "true" ]]; then
        log_dry "Executaria: mkdir -p ${dir_path} && chown ${owner} ${dir_path} && chmod ${permissions} ${dir_path}"
        return 0
    fi

    if [[ ! -d "${dir_path}" ]]; then
        mkdir -p "${dir_path}"
    fi

    chown "${owner}" "${dir_path}"
    chmod "${permissions}" "${dir_path}"
}

deploy_systemd_unit() {
    local service_name="$1"
    local source_file="$2"
    local dry_run="${3:-false}"

    log_step "Instalando serviço systemd: ${service_name}..."

    if [[ "${dry_run}" == "true" ]]; then
        log_dry "Executaria: cp ${source_file} /etc/systemd/system/${service_name} && systemctl daemon-reload"
        return 0
    fi

    cp "${source_file}" "/etc/systemd/system/${service_name}"
    chmod 644 "/etc/systemd/system/${service_name}"
    systemctl daemon-reload
    log_success "Unidade systemd ${service_name} instalada e daemon recarregado."
}

enable_and_start_service() {
    local service_name="$1"
    local dry_run="${2:-false}"

    log_step "Ativando e iniciando serviço: ${service_name}..."

    if [[ "${dry_run}" == "true" ]]; then
        log_dry "Executaria: systemctl enable ${service_name} && systemctl restart ${service_name}"
        return 0
    fi

    systemctl enable "${service_name}"
    systemctl restart "${service_name}"

    sleep 2
    if systemctl is-active --quiet "${service_name}"; then
        log_success "Serviço '${service_name}' está em execução ativa (active/running)."
    else
        log_error "Serviço '${service_name}' falhou ao iniciar. Verifique: journalctl -u ${service_name} -n 30"
        return 1
    fi
}
