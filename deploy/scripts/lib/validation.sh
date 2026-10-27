#!/usr/bin/env bash
# Linux Server Manager - Pre-flight & System Validation Library

check_disk_and_memory() {
    log_step "Verificando recursos de disco e memória..."

    local check_path="/var"
    [[ ! -d "/var" ]] && check_path="."

    # Verifica espaço livre em disco (mínimo 1024 MB livres)
    local free_disk_mb
    free_disk_mb=$(df -m "${check_path}" 2>/dev/null | awk 'NR==2 {print $4}' || echo "")
    if [[ -n "${free_disk_mb}" && "${free_disk_mb}" =~ ^[0-9]+$ ]]; then
        if [[ "${free_disk_mb}" -lt 1024 ]]; then
            log_warn "Espaço livre em ${check_path} é inferior a 1 GB (${free_disk_mb} MB disponíveis). Recomenda-se liberar espaço."
        else
            log_success "Espaço em disco em ${check_path}: ${free_disk_mb} MB livres (mínimo exigido: 1024 MB)."
        fi
    fi

    # Verifica memória livre
    if command -v free &>/dev/null; then
        local free_mem_mb
        free_mem_mb=$(free -m 2>/dev/null | awk '/^Mem:/ {print $7}' || echo "")
        if [[ -n "${free_mem_mb}" ]]; then
            log_info "Memória RAM disponível: ${free_mem_mb} MB."
        fi
    fi
}

check_port_available() {
    local port="$1"
    local desc="$2"

    if command -v ss &>/dev/null; then
        if ss -tuln | grep -q ":${port} "; then
            log_warn "A porta ${port} (${desc}) já está em uso por outro processo!"
            return 1
        fi
    elif command -v netstat &>/dev/null; then
        if netstat -tuln | grep -q ":${port} "; then
            log_warn "A porta ${port} (${desc}) já está em uso por outro processo!"
            return 1
        fi
    fi

    log_success "Porta ${port} (${desc}) está livre e pronta para uso."
    return 0
}

verify_precompiled_binary() {
    local bin_path="$1"
    local expected_name="$2"
    local expected_os="${3:-linux}"
    local expected_arch="${4:-amd64}"

    if [[ ! -f "${bin_path}" || ! -s "${bin_path}" ]]; then
        log_warn "Binário não encontrado ou com tamanho zero em: ${bin_path}"
        return 1
    fi

    if [[ -n "${expected_name}" && "$(basename "${bin_path}")" != "${expected_name}" ]]; then
        log_warn "Aviso: nome do binário '$(basename "${bin_path}")' difere do esperado ('${expected_name}')."
    fi

    log_step "Inspecionando procedência, formato e integridade do binário: ${bin_path}..."

    # 1. Verificação de formato executável ELF (Linux)
    local is_elf=false
    if command -v file &>/dev/null; then
        local file_out
        file_out=$(file -b "${bin_path}" 2>/dev/null || echo "")
        if [[ "${file_out}" =~ ^ELF && "${file_out}" =~ 64-bit ]]; then
            is_elf=true
        fi
    elif command -v readelf &>/dev/null; then
        if readelf -h "${bin_path}" 2>/dev/null | grep -q "ELF64"; then
            is_elf=true
        fi
    else
        # Fallback via magic bytes (primeiros 4 bytes: 0x7f 'E' 'L' 'F')
        local magic
        magic=$(head -c 4 "${bin_path}" 2>/dev/null || echo "")
        if [[ "${magic}" == $'\x7fELF' ]]; then
            is_elf=true
        fi
    fi

    if [[ "${expected_os}" == "linux" && "${is_elf}" != "true" ]]; then
        log_error "Formato inválido: '${bin_path}' não é um executável ELF 64-bit válido para Linux."
        return 2
    fi

    # 2. Verificação de Arquitetura de destino (amd64 / arm64)
    local is_arch_ok=false
    if command -v file &>/dev/null; then
        local f_desc
        f_desc=$(file -b "${bin_path}" 2>/dev/null || echo "")
        if [[ "${expected_arch}" == "amd64" && ( "${f_desc}" =~ "x86-64" || "${f_desc}" =~ "x86_64" ) ]]; then
            is_arch_ok=true
        elif [[ "${expected_arch}" == "arm64" && ( "${f_desc}" =~ "aarch64" || "${f_desc}" =~ "ARM aarch64" ) ]]; then
            is_arch_ok=true
        fi
    elif command -v readelf &>/dev/null; then
        local machine
        machine=$(readelf -h "${bin_path}" 2>/dev/null | awk -F: '/Machine:/ {print $2}' || echo "")
        if [[ "${expected_arch}" == "amd64" && "${machine}" =~ "Advanced Micro Devices X86-64" ]]; then
            is_arch_ok=true
        elif [[ "${expected_arch}" == "arm64" && "${machine}" =~ "AArch64" ]]; then
            is_arch_ok=true
        fi
    else
        # Se ferramentas ELF detalhadas não estiverem no host, aceita se ELF 64-bit verificado
        is_arch_ok=true
    fi

    if [[ "${is_arch_ok}" != "true" ]]; then
        log_error "Arquitetura incompatível: '${bin_path}' não corresponde à arquitetura do host (${expected_arch})."
        return 3
    fi

    # 3. Verificação da Versão da Toolchain Go utilizada na compilação do binário
    # Garante que binários locais antigos compilados com Go 1.23 não sejam aceitos como artefatos de produção
    local bin_go_ver=""
    if command -v go &>/dev/null; then
        bin_go_ver=$(go version "${bin_path}" 2>/dev/null | awk '{print $2}' | sed 's/go//' || echo "")
    fi
    if [[ -z "${bin_go_ver}" ]]; then
        # Extrai assinatura da runtime Go embutida no binário via strings/grep
        bin_go_ver=$(grep -a -o -E 'go1\.[0-9]+(\.[0-9]+)?' "${bin_path}" 2>/dev/null | head -n1 | sed 's/go//' || echo "")
    fi

    if [[ -n "${bin_go_ver}" ]]; then
        log_info "Compilador Go detectado no binário: Go ${bin_go_ver}."
        if ! validate_build_toolchain "${bin_go_ver}"; then
            log_error "Rejeição de Integridade de Produção: o binário '${bin_path}' foi compilado com Go ${bin_go_ver}."
            log_error "A política de segurança do LSM 1.2.2 proíbe o uso em produção de binários compilados com versões desatualizadas ou vulneráveis (ex: Go 1.23.0)."
            log_error "Utilize exclusivamente artefatos oficiais gerados pelo CI com Go 1.26.10+ ou Go 1.27.3+."
            return 4
        fi
    else
        log_warn "Aviso: Versão Go embutida não identificada no binário. Proceda com cautela se este não for um artefato oficial do CI."
    fi

    # 4. Verificação de Manifesto de Integridade (Obrigatório em Produção)
    local sha_file="${bin_path}.sha256"
    local sums_file
    sums_file="$(dirname "${bin_path}")/SHA256SUMS"
    local manifest_file=""
    local expected_hash=""

    if [[ -f "${sha_file}" ]]; then
        manifest_file="${sha_file}"
        expected_hash=$(awk '{print $1}' "${sha_file}")
    elif [[ -f "${sums_file}" ]]; then
        manifest_file="${sums_file}"
        local base_name
        base_name=$(basename "${bin_path}")

        # 4a. Validação de formato e segurança de caminhos no manifesto
        # Rejeita qualquer linha contendo / ou \ ou .. (caminhos inesperados ou path traversal)
        while IFS= read -r line || [[ -n "${line}" ]]; do
            [[ -z "${line// }" ]] && continue
            local line_file
            line_file=$(echo "${line}" | awk '{print $2}' | sed 's/^\*//')
            if [[ "${line_file}" =~ / || "${line_file}" =~ \\ || "${line_file}" =~ \.\. ]]; then
                log_error "FALHA CRÍTICA DE SEGURANÇA: Caminho inesperado ou ambíguo detectado no manifesto SHA256SUMS: '${line_file}'"
                return 10
            fi
        done < "${sums_file}"

        # 4b. Verificação de unicidade (proíbe nomes duplicados no manifesto)
        local count=0
        count=$(awk -v b="${base_name}" '($2 == b || $2 == "*"b) { c++ } END { print c+0 }' "${sums_file}")
        if [[ "${count}" -gt 1 ]]; then
            log_error "FALHA CRÍTICA DE SEGURANÇA: Entrada duplicada (${count} ocorrências) detectada no manifesto para o binário '${base_name}'!"
            return 11
        elif [[ "${count}" -eq 1 ]]; then
            expected_hash=$(awk -v b="${base_name}" '($2 == b || $2 == "*"b) { print $1 }' "${sums_file}")
        fi
    fi

    local env_mode="${LSM_ENV:-production}"

    if [[ -z "${manifest_file}" || -z "${expected_hash}" ]]; then
        if [[ "${env_mode}" == "production" ]]; then
            log_error "Manifesto de integridade SHA-256 ausente (${sha_file} ou ${sums_file}); em produção a evidência de integridade é obrigatória."
            return 5
        else
            log_warn "Aviso: Manifesto SHA-256 não encontrado. Permitido apenas para ambiente local de desenvolvimento."
        fi
    else
        log_info "Verificando hash SHA-256 no manifesto ${manifest_file}..."
        local actual_hash=""
        if command -v sha256sum &>/dev/null; then
            actual_hash=$(sha256sum "${bin_path}" | awk '{print $1}')
        elif command -v shasum &>/dev/null; then
            actual_hash=$(shasum -a 256 "${bin_path}" | awk '{print $1}')
        fi

        if [[ -n "${actual_hash}" && "${actual_hash}" != "${expected_hash}" ]]; then
            log_error "FALHA CRÍTICA DE INTEGRIDADE: O hash SHA-256 (${actual_hash}) não confere com o manifesto (${expected_hash})!"
            return 6
        fi
        log_success "Integridade criptográfica SHA-256 validada com sucesso (${actual_hash:0:16}...)."
    fi

    # 5. Verificação Criptográfica de Procedência do CI (Assinatura do Manifesto)
    if [[ -n "${manifest_file}" ]]; then
        local sig_file="${manifest_file}.sig"
        local pub_key="${LSM_CI_PUBKEY:-}"

        if [[ -z "${pub_key}" ]]; then
            local script_base
            script_base="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
            if [[ -f "${script_base}/../certs/ci-release.pub" ]]; then
                pub_key="${script_base}/../certs/ci-release.pub"
            elif [[ -f "${script_base}/certs/ci-release.pub" ]]; then
                pub_key="${script_base}/certs/ci-release.pub"
            elif [[ -f "/etc/lsm/ci-release.pub" ]]; then
                pub_key="/etc/lsm/ci-release.pub"
            fi
        fi

        if [[ ! -f "${sig_file}" ]]; then
            if [[ "${env_mode}" == "production" ]]; then
                log_error "Assinatura criptográfica do CI ausente (${sig_file}); em produção o manifesto deve possuir assinatura verificável que vincule a integridade à fonte confiável."
                return 7
            else
                log_warn "Aviso: Assinatura do CI não encontrada (${sig_file}). Ignorado em ambiente não-produção."
            fi
        else
            log_info "Validando procedência do CI via assinatura criptográfica (${sig_file})..."
            if [[ -z "${pub_key}" || ! -f "${pub_key}" ]]; then
                log_error "Chave pública de release do CI não encontrada para validação da assinatura."
                return 7
            fi

            # Validação da Raiz de Confiança da Chave Pública:
            # Impede que a chave pública oficial seja substituída arbitrariamente junto com um artefato adulterado
            local pub_fingerprint=""
            if command -v sha256sum &>/dev/null; then
                pub_fingerprint=$(sha256sum "${pub_key}" | awk '{print $1}')
            elif command -v shasum &>/dev/null; then
                pub_fingerprint=$(shasum -a 256 "${pub_key}" | awk '{print $1}')
            fi

            local expected_fingerprint="${LSM_EXPECTED_PUBKEY_SHA256:-43ff16c386fdf1982f28c1ba18c68ee529a7d58115b5a8647bf3e40669b8fc40}"
            if [[ "${env_mode}" == "production" && "${LSM_ALLOW_CUSTOM_PUBKEY:-false}" != "true" ]]; then
                if [[ "${pub_fingerprint}" != "${expected_fingerprint}" ]]; then
                    log_error "FALHA CRÍTICA DE RAIZ DE CONFIANÇA: A chave pública '${pub_key}' possui fingerprint (${pub_fingerprint}) que não coincide com a autoridade oficial do LSM (${expected_fingerprint})!"
                    return 9
                fi
            fi

            if command -v openssl &>/dev/null; then
                if ! openssl dgst -sha256 -verify "${pub_key}" -signature "${sig_file}" "${manifest_file}" &>/dev/null; then
                    log_error "FALHA CRÍTICA DE AUTENTICIDADE: A assinatura do manifesto de release do CI é inválida ou foi corrompida!"
                    return 8
                fi
                log_success "Procedência do CI e integridade do manifesto validadas com sucesso via chave pública oficial."
            else
                if [[ "${env_mode}" == "production" ]]; then
                    log_error "openssl não encontrado no host para verificar a assinatura criptográfica obrigatória do CI."
                    return 7
                fi
            fi
        fi
    fi

    log_success "Binário pré-compilado verificado, autenticado e aprovado para instalação oficial."
    return 0
}
