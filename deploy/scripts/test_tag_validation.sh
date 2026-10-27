#!/usr/bin/env bash
# Linux Server Manager - Teste Automatizado de Validação da Raiz de Confiança de Tags e Evidência de CI (LSM 1.2.2)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/lib/logging.sh
source "${SCRIPT_DIR}/lib/logging.sh"

echo "=== Iniciando Testes de Validação da Raiz de Confiança de Tags, Linhagem e Evidência de CI ==="

# Cria ambiente isolado temporário
TMP_TEST_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_TEST_DIR}"' EXIT

# Configura wrapper para GPG para compatibilidade com Git for Windows (MSYS2 path translation)
GPG_WRAPPER="${TMP_TEST_DIR}/gpg_wrap.sh"
cat <<'EOF' > "${GPG_WRAPPER}"
#!/bin/sh
if command -v cygpath >/dev/null 2>&1 && [ -n "$GNUPGHOME" ]; then
    GNUPGHOME=$(cygpath -u "$GNUPGHOME")
    export GNUPGHOME
fi
exec gpg "$@"
EOF
chmod +x "${GPG_WRAPPER}"

GNUPG_ROOT="${TMP_TEST_DIR}/gnupg"
mkdir -p "${GNUPG_ROOT}"
chmod 700 "${GNUPG_ROOT}"

if command -v cygpath >/dev/null 2>&1; then
    GNUPG_ROOT_POSIX=$(cygpath -u "${GNUPG_ROOT}")
else
    GNUPG_ROOT_POSIX="${GNUPG_ROOT}"
fi
export GNUPGHOME="${GNUPG_ROOT_POSIX}"

# 1. Gerar chave GPG de Mantenedor Confiável (Oficial)
cat <<EOF > "${TMP_TEST_DIR}/gpg_trusted.batch"
%no-protection
Key-Type: RSA
Key-Length: 2048
Subkey-Type: RSA
Subkey-Length: 2048
Name-Real: LSM Official Maintainer
Name-Email: maintainer@linuxservermanager.local
Expire-Date: 0
%commit
EOF

echo "Gerando chave GPG para o mantenedor oficial de teste..."
gpg --batch --generate-key "${TMP_TEST_DIR}/gpg_trusted.batch" 2>/dev/null
TRUSTED_KEY_ID=$(gpg --list-secret-keys --keyid-format LONG maintainer@linuxservermanager.local | awk '/sec/{print $2}' | cut -d/ -f2)
gpg --armor --export "${TRUSTED_KEY_ID}" > "${TMP_TEST_DIR}/trusted_maintainer.pub"

# 2. Gerar chave GPG de Atacante / Terceiro Não Autorizado
cat <<EOF > "${TMP_TEST_DIR}/gpg_attacker.batch"
%no-protection
Key-Type: RSA
Key-Length: 2048
Subkey-Type: RSA
Subkey-Length: 2048
Name-Real: Attacker Untrusted
Name-Email: attacker@evil.local
Expire-Date: 0
%commit
EOF

echo "Gerando chave GPG de terceiro não autorizado..."
gpg --batch --generate-key "${TMP_TEST_DIR}/gpg_attacker.batch" 2>/dev/null
ATTACKER_KEY_ID=$(gpg --list-secret-keys --keyid-format LONG attacker@evil.local | awk '/sec/{print $2}' | cut -d/ -f2)
gpg --armor --export "${ATTACKER_KEY_ID}" > "${TMP_TEST_DIR}/attacker.pub"

# 3. Inicializar repositório Git de teste com branch protegida main
REPO_DIR="${TMP_TEST_DIR}/repo"
mkdir -p "${REPO_DIR}"
cd "${REPO_DIR}"
git init -q
git config user.name "LSM Maintainer"
git config user.email "maintainer@linuxservermanager.local"
git config gpg.program "${GPG_WRAPPER}"

# Commit base na branch padrão protegida main
mkdir -p deploy/scripts/certs
cp "${TMP_TEST_DIR}/trusted_maintainer.pub" deploy/scripts/certs/maintainers.pub
git add .
git commit -q -m "Initial commit on main with trusted maintainer key"
git branch -M main

# Simula remote origin apontando para o próprio repositório
git remote add origin "${REPO_DIR}"
git update-ref refs/remotes/origin/main refs/heads/main

# Função de verificação de check-runs de CI (espelha exatamente o algoritmo de release.yml)
verify_ci_check_runs() {
    local target_sha="$1"
    local paginated_runs="$2"

    if [[ -z "${paginated_runs// }" ]]; then
        return 8 # Nenhuma verificação de CI encontrada
    fi

    local required_checks=(
        "Control Plane (Go 1.26.10)"
        "Control Plane (Go 1.27.3)"
        "Server Agent (Go 1.26.10)"
        "Server Agent (Go 1.27.3)"
        "Frontend Web (Node.js 20)"
        "Shell Scripts Lint & Validation"
    )

    local -A success_counts=()
    for req in "${required_checks[@]}"; do
        success_counts["${req}"]=0
    done

    local total_processed=0
    local any_failure=false

    while IFS=$'\t' read -r check_name check_sha check_status check_conclusion check_app || [[ -n "${check_name}" ]]; do
        [[ -z "${check_name}" ]] && continue

        if [[ "${check_name}" == "Release Pipeline" || "${check_name}" == "Verify Release Tag, Commit Lineage & CI Evidence" ]]; then
            continue
        fi

        total_processed=$((total_processed + 1))

        # 1. Valida correspondência de SHA com o commit-alvo
        if [[ "${check_sha}" != "${target_sha}" ]]; then
            return 9 # Divergência de head_sha
        fi

        # 2. Rejeita se qualquer verificação falhou, foi cancelada ou deu timeout
        if [[ "${check_conclusion}" == "failure" || "${check_conclusion}" == "cancelled" || "${check_conclusion}" == "timed_out" ]]; then
            any_failure=true
        fi

        # 3. Rejeita verificações incompletas ou em andamento
        if [[ "${check_status}" != "completed" ]]; then
            return 7 # Check em andamento
        fi

        # 4. Apenas o aplicativo oficial 'github-actions' pode satisfazer os checks obrigatórios
        if [[ "${check_app}" != "github-actions" ]]; then
            continue
        fi

        # 5. Registra aprovação de checks obrigatórios
        for req in "${required_checks[@]}"; do
            if [[ "${check_name}" == "${req}" ]]; then
                if [[ "${check_status}" == "completed" && "${check_conclusion}" == "success" ]]; then
                    success_counts["${req}"]=$(( success_counts["${req}"] + 1 ))
                else
                    any_failure=true
                fi
            fi
        done
    done <<< "${paginated_runs}"

    if [[ "${any_failure}" == "true" ]]; then
        return 6 # Check com falha
    fi

    if [[ "${total_processed}" -eq 0 ]]; then
        return 8 # Nenhum check processado
    fi

    local missing=0
    for req in "${required_checks[@]}"; do
        if [[ "${success_counts[${req}]}" -eq 0 ]]; then
            missing=$((missing + 1))
        fi
    done

    if [[ "${missing}" -gt 0 ]]; then
        return 10 # Check obrigatório ausente
    fi

    return 0
}

# Função de verificação completa que espelha o job verify-tag-and-commit do release.yml
run_verify_tag() {
    local tag_name="$1"
    local use_secret="$2" # "true" ou "false"
    local force_empty_root="${3:-false}"
    local paginated_runs="${4:-}"
    local sim_default_branch="${5:-main}"
    local sim_workflow_ref="${6:-main}"

    # 0. Validação da branch padrão autorizada e da referência de execução
    if [[ "${sim_default_branch}" != "main" && "${sim_default_branch}" != "master" ]]; then
        return 11 # Branch padrão não autorizada ou impossível de determinar
    fi

    if [[ "${sim_workflow_ref}" != "${sim_default_branch}" ]]; then
        return 12 # Workflow executado a partir de branch diferente da branch padrão
    fi

    # 1. Verifica se a tag é anotada (rejeita lightweight tags)
    local tag_type
    tag_type=$(git cat-file -t "${tag_name}" 2>/dev/null || true)
    if [[ "${tag_type}" != "tag" ]]; then
        return 1
    fi

    # 2. Resolução da raiz de confiança INDEPENDENTE da tag não confiável
    local maintainer_key=""
    if [[ "${force_empty_root}" == "true" ]]; then
        maintainer_key=""
    elif [[ "${use_secret}" == "true" ]]; then
        maintainer_key=$(cat "${TMP_TEST_DIR}/trusted_maintainer.pub")
    else
        # Busca estritamente da branch protegida origin/${sim_default_branch}
        maintainer_key=$(git show "origin/${sim_default_branch}:deploy/scripts/certs/maintainers.pub" 2>/dev/null || true)
    fi

    if [[ -z "${maintainer_key}" ]] || ! echo "${maintainer_key}" | grep -q -- "-----BEGIN PGP PUBLIC KEY BLOCK-----"; then
        return 2 # Falha por falta de raiz de confiança independente
    fi

    # Keyring efêmero para teste da verificação
    local verify_home="${TMP_TEST_DIR}/verify_gnupg_${tag_name}"
    rm -rf "${verify_home}"
    mkdir -p "${verify_home}"
    chmod 700 "${verify_home}"

    local verify_posix
    if command -v cygpath >/dev/null 2>&1; then
        verify_posix=$(cygpath -u "${verify_home}")
    else
        verify_posix="${verify_home}"
    fi

    local old_gnupg="${GNUPGHOME:-}"
    export GNUPGHOME="${verify_posix}"
    if ! printf '%s\n' "${maintainer_key}" | gpg --batch --import 2>/dev/null; then
        export GNUPGHOME="${old_gnupg}"
        return 3
    fi
    if ! git tag -v "${tag_name}" 2>/dev/null; then
        export GNUPGHOME="${old_gnupg}"
        return 4
    fi
    export GNUPGHOME="${old_gnupg}"

    # 3. Verifica se o commit apontado pela tag pertence EXCLUSIVAMENTE à linhagem da branch padrão protegida
    local tag_commit
    tag_commit=$(git rev-list -n 1 "${tag_name}")

    if ! git rev-parse --verify "origin/${sim_default_branch}" >/dev/null 2>&1; then
        return 11 # Branch padrão inexistente ou não verificável
    fi

    if ! git merge-base --is-ancestor "${tag_commit}" "origin/${sim_default_branch}"; then
        return 5 # Commit fora da linhagem da branch padrão protegida
    fi

    # 4. Validação estrita de evidência de CI aprovado para o commit exato
    verify_ci_check_runs "${tag_commit}" "${paginated_runs}"
}

# Helper para gerar payload de checks obrigatórios para um SHA
generate_valid_checks_payload() {
    local sha="$1"
    printf '%s\t%s\tcompleted\tsuccess\tgithub-actions\n' \
        "Control Plane (Go 1.26.10)" "${sha}" \
        "Control Plane (Go 1.27.3)" "${sha}" \
        "Server Agent (Go 1.26.10)" "${sha}" \
        "Server Agent (Go 1.27.3)" "${sha}" \
        "Frontend Web (Node.js 20)" "${sha}" \
        "Shell Scripts Lint & Validation" "${sha}"
}

echo "=== Executando Cenários de Auditoria e Testes Negativos ==="

# --- CASO 1: Rejeição de Tag Leve (lightweight) ---
echo -n "Testando Caso 1: Rejeição de Tag Leve (lightweight)... "
git tag v1.0.0-lightweight
set +e
run_verify_tag "v1.0.0-lightweight" "false"
case1_res=$?
set -e
if [[ "${case1_res}" -eq 1 ]]; then
    echo "[PASS] (código esperado: 1, obtido: 1 - rejeitada por não ser tag anotada)"
else
    echo "[FAIL] (esperado 1, obtido: ${case1_res})"
    exit 1
fi

# --- CASO 2: Rejeição de Tag Anotada sem assinatura ---
echo -n "Testando Caso 2: Rejeição de Tag Anotada sem assinatura... "
git tag -a v1.0.0-unsigned -m "Unsigned annotated tag"
set +e
run_verify_tag "v1.0.0-unsigned" "false"
case2_res=$?
set -e
if [[ "${case2_res}" -eq 4 ]]; then
    echo "[PASS] (código esperado: 4, obtido: 4 - rejeitada por ausência de assinatura GPG)"
else
    echo "[FAIL] (esperado 4, obtido: ${case2_res})"
    exit 1
fi

# --- CASO 3: Rejeição de Tag assinada por chave não confiável ---
echo -n "Testando Caso 3: Rejeição de Tag assinada por chave não confiável... "
git -c user.signingkey="${ATTACKER_KEY_ID}" tag -u "${ATTACKER_KEY_ID}" v1.0.0-attacker -m "Attacker signed tag"
set +e
run_verify_tag "v1.0.0-attacker" "false"
case3_res=$?
set -e
if [[ "${case3_res}" -eq 4 ]]; then
    echo "[PASS] (código esperado: 4, obtido: 4 - chave não presente na raiz de confiança)"
else
    echo "[FAIL] (esperado 4, obtido: ${case3_res})"
    exit 1
fi

# --- CASO 4: Rejeição quando chave não está no keyring efêmero ---
echo -n "Testando Caso 4: Rejeição de verificação sem correspondência de chave... "
git -c user.signingkey="${TRUSTED_KEY_ID}" tag -u "${TRUSTED_KEY_ID}" v1.0.0-signed-test -m "Signed tag test"
corrupt_home="${TMP_TEST_DIR}/empty_verify_gnupg"
mkdir -p "${corrupt_home}"
chmod 700 "${corrupt_home}"
if command -v cygpath >/dev/null 2>&1; then
    target_corrupt_home=$(cygpath -u "${corrupt_home}")
else
    target_corrupt_home="${corrupt_home}"
fi
set +e
GNUPGHOME="${target_corrupt_home}" git tag -v v1.0.0-signed-test 2>/dev/null
case4_res=$?
set -e
if [[ "${case4_res}" -ne 0 ]]; then
    echo "[PASS] (código esperado: != 0, obtido: ${case4_res} - rejeitada quando keyring não possui a chave)"
else
    echo "[FAIL] (esperado falha na assinatura, obtido: ${case4_res})"
    exit 1
fi

# --- CASO 5: Commit existente apenas numa branch alternativa (rejeitado por não pertencer a main) ---
echo -n "Testando Caso 5: Rejeição de commit existente apenas numa branch alternativa (v1.2.2/feature)... "
git checkout -q -b v1.2.2-alternative-branch
echo "alternative fix" > alternative.txt
git add alternative.txt
git commit -q -m "Commit on alternative branch only"
git update-ref refs/remotes/origin/v1.2.2 refs/heads/v1.2.2-alternative-branch
ALT_COMMIT=$(git rev-parse HEAD)
git -c user.signingkey="${TRUSTED_KEY_ID}" tag -u "${TRUSTED_KEY_ID}" v1.2.2-alt-tag -m "Tag on alternative branch"
ALT_CHECKS=$(generate_valid_checks_payload "${ALT_COMMIT}")
set +e
run_verify_tag "v1.2.2-alt-tag" "false" "false" "${ALT_CHECKS}" "main" "main"
case5_res=$?
set -e
if [[ "${case5_res}" -eq 5 ]]; then
    echo "[PASS] (código esperado: 5, obtido: 5 - commit bloqueado por não pertencer à branch padrão protegida main)"
else
    echo "[FAIL] (esperado 5, obtido: ${case5_res})"
    exit 1
fi

# --- CASO 6: Branch padrão não autorizada ou indeterminável ---
echo -n "Testando Caso 6: Rejeição quando branch padrão é não autorizada (ex: develop)... "
git checkout -q main
VALID_COMMIT=$(git rev-parse HEAD)
git -c user.signingkey="${TRUSTED_KEY_ID}" tag -u "${TRUSTED_KEY_ID}" v1.2.2-std-tag -m "Tag on main branch"
VALID_CHECKS=$(generate_valid_checks_payload "${VALID_COMMIT}")
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${VALID_CHECKS}" "develop" "develop"
case6_res=$?
set -e
if [[ "${case6_res}" -eq 11 ]]; then
    echo "[PASS] (código esperado: 11, obtido: 11 - rejeitado por branch padrão não autorizada)"
else
    echo "[FAIL] (esperado 11, obtido: ${case6_res})"
    exit 1
fi

# --- CASO 7: Workflow disparado a partir de branch diferente da branch padrão ---
echo -n "Testando Caso 7: Rejeição quando workflow é executado a partir de branch não padrão... "
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${VALID_CHECKS}" "main" "feature/my-branch"
case7_res=$?
set -e
if [[ "${case7_res}" -eq 12 ]]; then
    echo "[PASS] (código esperado: 12, obtido: 12 - execução fora da branch padrão bloqueada)"
else
    echo "[FAIL] (esperado 12, obtido: ${case7_res})"
    exit 1
fi

# --- CASO 8: Tentativa de ataque de confiança circular (chave adulterada no próprio commit) ---
echo -n "Testando Caso 8: Rejeição de tentativa de raiz circular (chave adulterada no próprio commit)... "
git checkout -q -b circular-attack-branch
cp "${TMP_TEST_DIR}/attacker.pub" deploy/scripts/certs/maintainers.pub
git add deploy/scripts/certs/maintainers.pub
git commit -q -m "Attacker attempts to replace maintainers.pub in tag tree"
git -c user.signingkey="${ATTACKER_KEY_ID}" tag -u "${ATTACKER_KEY_ID}" v1.0.0-circular-attack -m "Circular attack tag"
set +e
# A verificação busca da branch protegida origin/main e ignora a árvore da tag
run_verify_tag "v1.0.0-circular-attack" "false"
case8_res=$?
set -e
if [[ "${case8_res}" -eq 4 ]]; then
    echo "[PASS] (código esperado: 4, obtido: 4 - ataque neutralizado: chave lida de origin/main)"
else
    echo "[FAIL] (esperado 4, obtido: ${case8_res})"
    exit 1
fi

# --- CASO 9: Ausência total de raiz de confiança independente ---
echo -n "Testando Caso 9: Falha bloqueante por ausência de raiz de confiança independente... "
set +e
run_verify_tag "v1.0.0-circular-attack" "false" "true"
case9_res=$?
set -e
if [[ "${case9_res}" -eq 2 ]]; then
    echo "[PASS] (código esperado: 2, obtido: 2 - erro bloqueante por falta de chave independente)"
else
    echo "[FAIL] (esperado 2, obtido: ${case9_res})"
    exit 1
fi

# --- CASO 10: Ausência de um check obrigatório do CI ---
echo -n "Testando Caso 10: Rejeição de CI com check obrigatório ausente (5 de 6 checks)... "
INCOMPLETE_CHECKS=$(printf '%s\t%s\tcompleted\tsuccess\tgithub-actions\n' \
    "Control Plane (Go 1.26.10)" "${VALID_COMMIT}" \
    "Control Plane (Go 1.27.3)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.26.10)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.27.3)" "${VALID_COMMIT}" \
    "Frontend Web (Node.js 20)" "${VALID_COMMIT}")
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${INCOMPLETE_CHECKS}" "main" "main"
case10_res=$?
set -e
if [[ "${case10_res}" -eq 10 ]]; then
    echo "[PASS] (código esperado: 10, obtido: 10 - rejeitado por check obrigatório 'Shell Scripts Lint & Validation' ausente)"
else
    echo "[FAIL] (esperado 10, obtido: ${case10_res})"
    exit 1
fi

# --- CASO 11: Check obrigatório com falha ---
echo -n "Testando Caso 11: Rejeição de CI quando um check obrigatório falha... "
FAILED_CHECKS=$(printf '%s\t%s\tcompleted\tsuccess\tgithub-actions\n' \
    "Control Plane (Go 1.26.10)" "${VALID_COMMIT}" \
    "Control Plane (Go 1.27.3)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.26.10)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.27.3)" "${VALID_COMMIT}" \
    "Frontend Web (Node.js 20)" "${VALID_COMMIT}"
    printf '%s\t%s\tcompleted\tfailure\tgithub-actions\n' \
    "Shell Scripts Lint & Validation" "${VALID_COMMIT}")
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${FAILED_CHECKS}" "main" "main"
case11_res=$?
set -e
if [[ "${case11_res}" -eq 6 ]]; then
    echo "[PASS] (código esperado: 6, obtido: 6 - rejeitado por check com falha)"
else
    echo "[FAIL] (esperado 6, obtido: ${case11_res})"
    exit 1
fi

# --- CASO 12: Check obrigatório ainda em execução ---
echo -n "Testando Caso 12: Rejeição de CI com check ainda em andamento... "
PENDING_CHECKS=$(printf '%s\t%s\tcompleted\tsuccess\tgithub-actions\n' \
    "Control Plane (Go 1.26.10)" "${VALID_COMMIT}" \
    "Control Plane (Go 1.27.3)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.26.10)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.27.3)" "${VALID_COMMIT}" \
    "Frontend Web (Node.js 20)" "${VALID_COMMIT}"
    printf '%s\t%s\tin_progress\tnone\tgithub-actions\n' \
    "Shell Scripts Lint & Validation" "${VALID_COMMIT}")
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${PENDING_CHECKS}" "main" "main"
case12_res=$?
set -e
if [[ "${case12_res}" -eq 7 ]]; then
    echo "[PASS] (código esperado: 7, obtido: 7 - rejeitado por check com status in_progress)"
else
    echo "[FAIL] (esperado 7, obtido: ${case12_res})"
    exit 1
fi

# --- CASO 13: Check de integração externa isolado sem os checks reais do CI ---
echo -n "Testando Caso 13: Rejeição de check externo isolado (codecov/sonar) sem os checks oficiais... "
EXTERNAL_CHECKS=$(printf '%s\t%s\tcompleted\tsuccess\tcodecov-bot\n' "Code Coverage Report" "${VALID_COMMIT}")
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${EXTERNAL_CHECKS}" "main" "main"
case13_res=$?
set -e
if [[ "${case13_res}" -eq 10 ]]; then
    echo "[PASS] (código esperado: 10, obtido: 10 - check externo não substitui os checks obrigatórios do CI oficial)"
else
    echo "[FAIL] (esperado 10, obtido: ${case13_res})"
    exit 1
fi

# --- CASO 14: Divergência de head_sha no check run ---
echo -n "Testando Caso 14: Rejeição de check run com head_sha divergente do commit-alvo... "
MISMATCHED_SHA_CHECKS=$(printf '%s\t%s\tcompleted\tsuccess\tgithub-actions\n' \
    "Control Plane (Go 1.26.10)" "0000000000000000000000000000000000000000" \
    "Control Plane (Go 1.27.3)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.26.10)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.27.3)" "${VALID_COMMIT}" \
    "Frontend Web (Node.js 20)" "${VALID_COMMIT}" \
    "Shell Scripts Lint & Validation" "${VALID_COMMIT}")
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${MISMATCHED_SHA_CHECKS}" "main" "main"
case14_res=$?
set -e
if [[ "${case14_res}" -eq 9 ]]; then
    echo "[PASS] (código esperado: 9, obtido: 9 - divergência de head_sha detectada e bloqueada)"
else
    echo "[FAIL] (esperado 9, obtido: ${case14_res})"
    exit 1
fi

# --- CASO 15: Resultados distribuídos por várias páginas da API (simulação de paginação agregada) ---
echo -n "Testando Caso 15: Validação com resultados distribuídos por várias páginas da API... "
# Simula agregação da Página 1 (3 checks) e Página 2 (3 checks)
PAGE1=$(printf '%s\t%s\tcompleted\tsuccess\tgithub-actions\n' \
    "Control Plane (Go 1.26.10)" "${VALID_COMMIT}" \
    "Control Plane (Go 1.27.3)" "${VALID_COMMIT}" \
    "Server Agent (Go 1.26.10)" "${VALID_COMMIT}")
PAGE2=$(printf '%s\t%s\tcompleted\tsuccess\tgithub-actions\n' \
    "Server Agent (Go 1.27.3)" "${VALID_COMMIT}" \
    "Frontend Web (Node.js 20)" "${VALID_COMMIT}" \
    "Shell Scripts Lint & Validation" "${VALID_COMMIT}")
PAGINATED_MULTIPLE_PAGES="${PAGE1}"$'\n'"${PAGE2}"
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${PAGINATED_MULTIPLE_PAGES}" "main" "main"
case15_res=$?
set -e
if [[ "${case15_res}" -eq 0 ]]; then
    echo "[PASS] (código esperado: 0, obtido: 0 - agregação de múltiplas páginas aprovada com sucesso)"
else
    echo "[FAIL] (esperado 0, obtido: ${case15_res})"
    exit 1
fi

# --- CASO 16: Sucesso total com tag assinada por mantenedor na branch padrão protegida com CI aprovado ---
echo -n "Testando Caso 16: Aceitação total na branch padrão com todos os 6 checks de CI aprovados... "
set +e
run_verify_tag "v1.2.2-std-tag" "false" "false" "${VALID_CHECKS}" "main" "main"
case16_res=$?
set -e
if [[ "${case16_res}" -eq 0 ]]; then
    echo "[PASS] (código esperado: 0, obtido: 0 - tag, linhagem e CI totalmente aprovados)"
else
    echo "[FAIL] (esperado 0, obtido: ${case16_res})"
    exit 1
fi

# --- CASO 17: Aceitação via Segredo Independente LSM_MAINTAINER_GPG_PUB ---
echo -n "Testando Caso 17: Aceitação de tag legítima via segredo independente... "
set +e
run_verify_tag "v1.2.2-std-tag" "true" "false" "${VALID_CHECKS}" "main" "main"
case17_res=$?
set -e
if [[ "${case17_res}" -eq 0 ]]; then
    echo "[PASS] (código esperado: 0, obtido: 0 - aprovada via segredo LSM_MAINTAINER_GPG_PUB)"
else
    echo "[FAIL] (esperado 0, obtido: ${case17_res})"
    exit 1
fi

echo "=== Todos os 17 testes de validação da raiz de confiança de tags, linhagem e evidência de CI foram APROVADOS! ==="
