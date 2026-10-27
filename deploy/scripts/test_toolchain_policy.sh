#!/usr/bin/env bash
# Linux Server Manager - Teste de Limites da Política de Toolchain Go e Integridade de Artefatos (LSM 1.2.2)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/lib/logging.sh
source "${SCRIPT_DIR}/lib/logging.sh"
# shellcheck source=deploy/scripts/lib/dependencies.sh
source "${SCRIPT_DIR}/lib/dependencies.sh"
# shellcheck source=deploy/scripts/lib/validation.sh
source "${SCRIPT_DIR}/lib/validation.sh"

echo "=== Iniciando Teste de Limites da Política de Toolchain Go ==="

test_version() {
    local ver="$1"
    local expected_code="$2"
    local desc="$3"

    echo -n "Testando Go ${ver} (${desc})... "

    set +e
    LSM_ALLOW_UNSUPPORTED_COMPILER="false" LSM_ENV="production" validate_build_toolchain "${ver}" >/dev/null 2>&1
    local actual_code=$?
    set -e

    if [[ "${actual_code}" -eq "${expected_code}" ]]; then
        echo "[PASS] (código esperado: ${expected_code}, obtido: ${actual_code})"
    else
        echo "[FAIL] (código esperado: ${expected_code}, obtido: ${actual_code})"
        return 1
    fi
}

# 1. Limite legado: Go 1.23.0 (vulnerável / fora de suporte na política de produção)
test_version "1.23.0" 2 "Legado vulnerável (deve ser rejeitado com código 2)"

# 2. Limite Go 1.26: Go 1.26.9 (patch anterior ao patch fix 1.26.10)
test_version "1.26.9" 2 "Go 1.26 sem patch de segurança (deve ser rejeitado com código 2)"

# 3. Limite Go 1.26: Go 1.26.10 (versão aprovada pelo CI com patch aplicado)
test_version "1.26.10" 0 "Go 1.26.10 com patch oficial (deve ser aceito com código 0)"

# 4. Limite Go 1.27: Go 1.27.2 (patch anterior ao patch fix 1.27.3)
test_version "1.27.2" 2 "Go 1.27 sem patch de segurança (deve ser rejeitado com código 2)"

# 5. Limite Go 1.27: Go 1.27.3 (versão aprovada pelo CI com patch aplicado)
test_version "1.27.3" 0 "Go 1.27.3 com patch oficial (deve ser aceito com código 0)"

# 6. Versão futura não homologada: Go 1.28.0 (rejeitado por não ser validado pela política)
test_version "1.28.0" 2 "Go 1.28 futuro não homologado (deve ser rejeitado com código 2)"

# 7. Pré-lançamento não estável: Go 1.26.10rc1 (rejeitado)
test_version "1.26.10rc1" 2 "Go 1.26.10rc1 pré-lançamento (deve ser rejeitado com código 2)"

# 8. Tentativa de contornar a política de produção com LSM_ALLOW_UNSUPPORTED_COMPILER=true
echo -n "Testando tentativa de contornar política com LSM_ALLOW_UNSUPPORTED_COMPILER=true em produção... "
set +e
LSM_ALLOW_UNSUPPORTED_COMPILER="true" LSM_ENV="production" validate_build_toolchain "1.23.0" >/dev/null 2>&1
bypass_prod_code=$?
set -e
if [[ "${bypass_prod_code}" -eq 2 ]]; then
    echo "[PASS] (código esperado: 2, obtido: 2)"
else
    echo "[FAIL] (esperado código 2 bloqueando bypass em produção, obtido: ${bypass_prod_code})"
    exit 1
fi

# 9. Teste de override permitido em ambiente de desenvolvimento (LSM_ENV=development)
echo -n "Testando Go 1.23.0 com LSM_ALLOW_UNSUPPORTED_COMPILER=true em desenvolvimento... "
set +e
LSM_ALLOW_UNSUPPORTED_COMPILER="true" LSM_ENV="development" validate_build_toolchain "1.23.0" >/dev/null 2>&1
override_code=$?
set -e
if [[ "${override_code}" -eq 0 ]]; then
    echo "[PASS] (código esperado: 0, obtido: 0)"
else
    echo "[FAIL] (esperado 0 no override de dev, obtido: ${override_code})"
    exit 1
fi

echo "=== Testando Validação de Procedência, Assinatura e Integridade de Artefatos ==="

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

# Gerar par de chaves RSA efêmero simulando autoridade de release do CI
openssl genrsa -out "${TMP_DIR}/test_ci_priv.pem" 2048 2>/dev/null
openssl rsa -in "${TMP_DIR}/test_ci_priv.pem" -pubout -out "${TMP_DIR}/test_ci_pub.pem" 2>/dev/null
TEST_PUBKEY_SHA256=$(sha256sum "${TMP_DIR}/test_ci_pub.pem" | awk '{print $1}')

# Teste 10: Artefato legítimo pré-compilado aprovado com manifesto e assinatura RSA
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${TMP_DIR}/lsm-legit-bin"
(
    cd "${TMP_DIR}"
    sha256sum "lsm-legit-bin" > "SHA256SUMS"
    openssl dgst -sha256 -sign "${TMP_DIR}/test_ci_priv.pem" -out "SHA256SUMS.sig" "SHA256SUMS"
)

echo -n "Testando artefato legítimo verificado e assinado pelo CI... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${TMP_DIR}/lsm-legit-bin" "lsm-legit-bin" "linux" "amd64" >/dev/null 2>&1
legit_code=$?
set -e
if [[ "${legit_code}" -eq 0 ]]; then
    echo "[PASS] (código esperado: 0, obtido: 0)"
else
    echo "[FAIL] (esperado 0 para artefato legítimo assinado, obtido: ${legit_code})"
    exit 1
fi

# Teste 11: Rejeição de executável Windows (PE começando com MZ)
printf 'MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00' > "${TMP_DIR}/fake-win.exe"
echo -n "Testando rejeição de executável Windows (PE)... "
set +e
LSM_ENV="production" verify_precompiled_binary "${TMP_DIR}/fake-win.exe" "fake-win.exe" "linux" "amd64" >/dev/null 2>&1
win_code=$?
set -e
if [[ "${win_code}" -eq 2 ]]; then
    echo "[PASS] (código esperado: 2, obtido: 2)"
else
    echo "[FAIL] (esperado 2 para binário Windows, obtido: ${win_code})"
    exit 1
fi

# Teste 12: Arquitetura incompatível (arm64 em host amd64)
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\xb7\x00\x01\x00\x00\x00go1.26.10-arm64' > "${TMP_DIR}/lsm-arm64-bin"
echo -n "Testando rejeição de arquitetura errada (arm64 para host amd64)... "
set +e
LSM_ENV="production" verify_precompiled_binary "${TMP_DIR}/lsm-arm64-bin" "lsm-arm64-bin" "linux" "amd64" >/dev/null 2>&1
arch_code=$?
set -e
if [[ "${arch_code}" -eq 3 ]]; then
    echo "[PASS] (código esperado: 3, obtido: 3)"
else
    echo "[FAIL] (esperado 3 para arquitetura incompatível, obtido: ${arch_code})"
    exit 1
fi

# Teste 13: Toolchain legada vulnerável embutida no binário ELF (Go 1.23.0)
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.23.0' > "${TMP_DIR}/lsm-vuln-bin"
echo -n "Testando rejeição de binário ELF com toolchain vulnerável (Go 1.23.0)... "
set +e
LSM_ENV="production" verify_precompiled_binary "${TMP_DIR}/lsm-vuln-bin" "lsm-vuln-bin" "linux" "amd64" >/dev/null 2>&1
vuln_bin_code=$?
set -e
if [[ "${vuln_bin_code}" -eq 4 ]]; then
    echo "[PASS] (código esperado: 4, obtido: 4)"
else
    echo "[FAIL] (esperado 4 para toolchain vulnerável, obtido: ${vuln_bin_code})"
    exit 1
fi

# Teste 14: Binário sem manifesto SHA-256 em ambiente de produção
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${TMP_DIR}/lsm-nomanifest"
echo -n "Testando rejeição de binário local sem manifesto em produção... "
set +e
LSM_ENV="production" verify_precompiled_binary "${TMP_DIR}/lsm-nomanifest" "lsm-nomanifest" "linux" "amd64" >/dev/null 2>&1
noman_code=$?
set -e
if [[ "${noman_code}" -eq 5 ]]; then
    echo "[PASS] (código esperado: 5, obtido: 5)"
else
    echo "[FAIL] (esperado 5 para manifesto ausente em produção, obtido: ${noman_code})"
    exit 1
fi

# Teste 15: Binário com hash divergente do manifesto
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10-tampered' > "${TMP_DIR}/lsm-tampered"
echo "0000000000000000000000000000000000000000000000000000000000000000  lsm-tampered" > "${TMP_DIR}/lsm-tampered.sha256"
echo -n "Testando rejeição de binário com hash incorreto... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${TMP_DIR}/lsm-tampered" "lsm-tampered" "linux" "amd64" >/dev/null 2>&1
hash_code=$?
set -e
if [[ "${hash_code}" -eq 6 ]]; then
    echo "[PASS] (código esperado: 6, obtido: 6)"
else
    echo "[FAIL] (esperado 6 para divergência de hash, obtido: ${hash_code})"
    exit 1
fi

# Teste 16: Manifesto sem assinatura criptográfica digital em produção
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${TMP_DIR}/lsm-unsigned"
(
    cd "${TMP_DIR}"
    sha256sum "lsm-unsigned" > "lsm-unsigned.sha256"
)
echo -n "Testando rejeição de manifesto sem assinatura em produção... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${TMP_DIR}/lsm-unsigned" "lsm-unsigned" "linux" "amd64" >/dev/null 2>&1
nosig_code=$?
set -e
if [[ "${nosig_code}" -eq 7 ]]; then
    echo "[PASS] (código esperado: 7, obtido: 7)"
else
    echo "[FAIL] (esperado 7 para assinatura ausente, obtido: ${nosig_code})"
    exit 1
fi

# Teste 17: Assinatura inválida ou corrompida
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${TMP_DIR}/lsm-badsig"
(
    cd "${TMP_DIR}"
    sha256sum "lsm-badsig" > "lsm-badsig.sha256"
    echo "CORRUPTED_SIGNATURE_BYTES" > "lsm-badsig.sha256.sig"
)
echo -n "Testando rejeição de assinatura inválida/corrompida... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${TMP_DIR}/lsm-badsig" "lsm-badsig" "linux" "amd64" >/dev/null 2>&1
badsig_code=$?
set -e
if [[ "${badsig_code}" -eq 8 ]]; then
    echo "[PASS] (código esperado: 8, obtido: 8)"
else
    echo "[FAIL] (esperado 8 para assinatura corrompida, obtido: ${badsig_code})"
    exit 1
fi

# Teste 18: Assinatura com chave pública não confiável (fingerprint divergente da autoridade oficial)
openssl genrsa -out "${TMP_DIR}/untrusted_priv.pem" 2048 2>/dev/null
openssl rsa -in "${TMP_DIR}/untrusted_priv.pem" -pubout -out "${TMP_DIR}/untrusted_pub.pem" 2>/dev/null
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${TMP_DIR}/lsm-untrusted"
(
    cd "${TMP_DIR}"
    sha256sum "lsm-untrusted" > "lsm-untrusted.sha256"
    openssl dgst -sha256 -sign "${TMP_DIR}/untrusted_priv.pem" -out "lsm-untrusted.sha256.sig" "lsm-untrusted.sha256"
)
echo -n "Testando rejeição de chave não confiável (fingerprint não homologado)... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/untrusted_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${TMP_DIR}/lsm-untrusted" "lsm-untrusted" "linux" "amd64" >/dev/null 2>&1
untrusted_code=$?
set -e
if [[ "${untrusted_code}" -eq 9 ]]; then
    echo "[PASS] (código esperado: 9, obtido: 9)"
else
    echo "[FAIL] (esperado 9 para fingerprint não homologado, obtido: ${untrusted_code})"
    exit 1
fi

# Teste 19: Caminho inesperado ou ambíguo no manifesto (tentativa de path traversal ../ ou /)
DIR_TEST19="${TMP_DIR}/test19"
mkdir -p "${DIR_TEST19}"
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${DIR_TEST19}/lsm-path-test"
echo "0000000000000000000000000000000000000000000000000000000000000000  ../lsm-path-test" > "${DIR_TEST19}/SHA256SUMS"
openssl dgst -sha256 -sign "${TMP_DIR}/test_ci_priv.pem" -out "${DIR_TEST19}/SHA256SUMS.sig" "${DIR_TEST19}/SHA256SUMS"
echo -n "Testando rejeição de caminho inesperado no manifesto (path traversal)... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${DIR_TEST19}/lsm-path-test" "lsm-path-test" "linux" "amd64" >/dev/null 2>&1
path_code=$?
set -e
if [[ "${path_code}" -eq 10 ]]; then
    echo "[PASS] (código esperado: 10, obtido: 10)"
else
    echo "[FAIL] (esperado 10 para caminho inesperado no manifesto, obtido: ${path_code})"
    exit 1
fi

# Teste 20: Entrada duplicada no manifesto SHA256SUMS para o mesmo binário
DIR_TEST20="${TMP_DIR}/test20"
mkdir -p "${DIR_TEST20}"
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${DIR_TEST20}/lsm-dup-test"
HASH_DUP=$(sha256sum "${DIR_TEST20}/lsm-dup-test" | awk '{print $1}')
echo "${HASH_DUP}  lsm-dup-test" > "${DIR_TEST20}/SHA256SUMS"
echo "${HASH_DUP}  lsm-dup-test" >> "${DIR_TEST20}/SHA256SUMS"
openssl dgst -sha256 -sign "${TMP_DIR}/test_ci_priv.pem" -out "${DIR_TEST20}/SHA256SUMS.sig" "${DIR_TEST20}/SHA256SUMS"
echo -n "Testando rejeição de nome de binário duplicado no manifesto... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${DIR_TEST20}/lsm-dup-test" "lsm-dup-test" "linux" "amd64" >/dev/null 2>&1
dup_code=$?
set -e
if [[ "${dup_code}" -eq 11 ]]; then
    echo "[PASS] (código esperado: 11, obtido: 11)"
else
    echo "[FAIL] (esperado 11 para entrada duplicada no manifesto, obtido: ${dup_code})"
    exit 1
fi

# Teste 21: Binário ausente no manifesto SHA256SUMS
DIR_TEST21="${TMP_DIR}/test21"
mkdir -p "${DIR_TEST21}"
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${DIR_TEST21}/lsm-missing-from-manifest"
echo "0000000000000000000000000000000000000000000000000000000000000000  other-binary" > "${DIR_TEST21}/SHA256SUMS"
openssl dgst -sha256 -sign "${TMP_DIR}/test_ci_priv.pem" -out "${DIR_TEST21}/SHA256SUMS.sig" "${DIR_TEST21}/SHA256SUMS"
echo -n "Testando rejeição de binário ausente no manifesto SHA256SUMS... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${DIR_TEST21}/lsm-missing-from-manifest" "lsm-missing-from-manifest" "linux" "amd64" >/dev/null 2>&1
missing_code=$?
set -e
if [[ "${missing_code}" -eq 5 ]]; then
    echo "[PASS] (código esperado: 5, obtido: 5)"
else
    echo "[FAIL] (esperado 5 para binário ausente no manifesto, obtido: ${missing_code})"
    exit 1
fi

# Teste 22: Binário substituído depois da assinatura do manifesto (divergência de hash)
DIR_TEST22="${TMP_DIR}/test22"
mkdir -p "${DIR_TEST22}"
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${DIR_TEST22}/lsm-swapped"
(
    cd "${DIR_TEST22}"
    sha256sum "lsm-swapped" > "SHA256SUMS"
    openssl dgst -sha256 -sign "${TMP_DIR}/test_ci_priv.pem" -out "SHA256SUMS.sig" "SHA256SUMS"
)
# Modifica o binário após a assinatura
echo "MODIFIED" >> "${DIR_TEST22}/lsm-swapped"
echo -n "Testando rejeição de binário substituído depois da assinatura... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${DIR_TEST22}/lsm-swapped" "lsm-swapped" "linux" "amd64" >/dev/null 2>&1
swapped_code=$?
set -e
if [[ "${swapped_code}" -eq 6 ]]; then
    echo "[PASS] (código esperado: 6, obtido: 6)"
else
    echo "[FAIL] (esperado 6 para binário adulterado pós-assinatura, obtido: ${swapped_code})"
    exit 1
fi

# Teste 23: Manifesto SHA256SUMS e assinatura incompatíveis
DIR_TEST23="${TMP_DIR}/test23"
mkdir -p "${DIR_TEST23}"
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${DIR_TEST23}/lsm-mismatch"
(
    cd "${DIR_TEST23}"
    sha256sum "lsm-mismatch" > "SHA256SUMS"
    echo "DIFFERENT_MANIFEST_FOR_SIGNATURE" > "OTHER_MANIFEST"
    openssl dgst -sha256 -sign "${TMP_DIR}/test_ci_priv.pem" -out "SHA256SUMS.sig" "OTHER_MANIFEST"
)
echo -n "Testando rejeição de manifesto e assinatura incompatíveis... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/test_ci_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${DIR_TEST23}/lsm-mismatch" "lsm-mismatch" "linux" "amd64" >/dev/null 2>&1
mismatch_code=$?
set -e
if [[ "${mismatch_code}" -eq 8 ]]; then
    echo "[PASS] (código esperado: 8, obtido: 8)"
else
    echo "[FAIL] (esperado 8 para assinatura incompatível com o manifesto, obtido: ${mismatch_code})"
    exit 1
fi

# Teste 24: Binário e manifesto alterados em conjunto sem chave confiável
DIR_TEST24="${TMP_DIR}/test24"
mkdir -p "${DIR_TEST24}"
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00\x01\x00\x00\x00go1.26.10' > "${DIR_TEST24}/lsm-attacker-bin"
(
    cd "${DIR_TEST24}"
    sha256sum "lsm-attacker-bin" > "SHA256SUMS"
    openssl dgst -sha256 -sign "${TMP_DIR}/untrusted_priv.pem" -out "SHA256SUMS.sig" "SHA256SUMS"
)
echo -n "Testando rejeição de binário e manifesto alterados juntos sem chave confiável... "
set +e
LSM_ENV="production" LSM_CI_PUBKEY="${TMP_DIR}/untrusted_pub.pem" LSM_EXPECTED_PUBKEY_SHA256="${TEST_PUBKEY_SHA256}" \
    verify_precompiled_binary "${DIR_TEST24}/lsm-attacker-bin" "lsm-attacker-bin" "linux" "amd64" >/dev/null 2>&1
attacker_code=$?
set -e
if [[ "${attacker_code}" -eq 9 ]]; then
    echo "[PASS] (código esperado: 9, obtido: 9)"
else
    echo "[FAIL] (esperado 9 para violação de autoridade oficial de chave, obtido: ${attacker_code})"
    exit 1
fi

echo "=== Todos os 24 testes de política Go, limites e proveniência de artefatos foram APROVADOS! ==="
