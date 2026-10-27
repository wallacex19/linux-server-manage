#!/usr/bin/env bash
# Linux Server Manager - Teste do Fluxo de Release sem Publicação (LSM 1.2.2)
# Valida geração de binários, manifesto SHA256SUMS, assinatura RSA-2048/SHA-256 e rejeições de integridade
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/lib/logging.sh
source "${SCRIPT_DIR}/lib/logging.sh"

echo "=== Iniciando Teste de Validação do Fluxo de Release (Dry-Run com Chaves Temporárias) ==="

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

# 1. Gerar par de chaves RSA-2048 efêmero para o teste do fluxo de assinatura
openssl genrsa -out "${TMP_DIR}/test_rsa_priv.pem" 2048 2>/dev/null
openssl rsa -in "${TMP_DIR}/test_rsa_priv.pem" -pubout -out "${TMP_DIR}/test_rsa_pub.pem" 2>/dev/null

# Gerar par de chaves RSA não autorizado (para teste de falsificação)
openssl genrsa -out "${TMP_DIR}/attacker_priv.pem" 2048 2>/dev/null
openssl rsa -in "${TMP_DIR}/attacker_priv.pem" -pubout -out "${TMP_DIR}/attacker_pub.pem" 2>/dev/null

TEST_PUB_FINGERPRINT=$(sha256sum "${TMP_DIR}/test_rsa_pub.pem" | awk '{print $1}')

# 2. Simular diretório dist com os quatro binários oficiais Linux
DIST_DIR="${TMP_DIR}/dist"
mkdir -p "${DIST_DIR}"

BINARIES=(
    "lsm-control-plane-linux-amd64"
    "lsm-control-plane-linux-arm64"
    "lsm-agent-linux-amd64"
    "lsm-agent-linux-arm64"
)

for b in "${BINARIES[@]}"; do
    # Gera cabeçalho ELF 64-bit mínimo simulado
    printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00' > "${DIST_DIR}/${b}"
    echo "Simulated binary payload for ${b} (LSM 1.2.2 release pipeline dry-run)" >> "${DIST_DIR}/${b}"
done

echo -n "Testando criação do manifesto canônico SHA256SUMS com os 4 binários oficiais... "
(
    cd "${DIST_DIR}"
    sha256sum "${BINARIES[@]}" > SHA256SUMS
)
# Confirma que exatamente os 4 arquivos estão no manifesto
MANIFEST_COUNT=$(wc -l < "${DIST_DIR}/SHA256SUMS")
if [[ "${MANIFEST_COUNT}" -eq 4 ]]; then
    echo "[PASS] (exatamente 4 binários oficiais registrados no manifesto canônico)"
else
    echo "[FAIL] (esperado 4 entradas no manifesto, obtido: ${MANIFEST_COUNT})"
    exit 1
fi

echo -n "Testando verificação prévia de integridade (sha256sum -c SHA256SUMS)... "
(
    cd "${DIST_DIR}"
    sha256sum -c SHA256SUMS >/dev/null 2>&1
)
echo "[PASS] (todos os 4 binários íntegros)"

echo -n "Testando assinatura criptográfica estrita RSA-2048 com SHA-256... "
sign_with_key() {
    local k_path="$1"
    local m_path="$2"
    local s_out="$3"
    # No Linux (Ubuntu 24.04 do CI), /dev/stdin é nativo do VFS e permite passagem da chave em memória
    if cat "${k_path}" | openssl dgst -sha256 -sign /dev/stdin -out "${s_out}" "${m_path}" 2>/dev/null; then
        return 0
    else
        # Fallback para ambientes locais Windows onde mingw64 openssl não possui suporte a /dev/stdin
        openssl dgst -sha256 -sign "${k_path}" -out "${s_out}" "${m_path}"
    fi
}

sign_with_key "${TMP_DIR}/test_rsa_priv.pem" "${DIST_DIR}/SHA256SUMS" "${DIST_DIR}/SHA256SUMS.sig"
if [[ -s "${DIST_DIR}/SHA256SUMS.sig" ]]; then
    echo "[PASS] (arquivo SHA256SUMS.sig gerado com sucesso via RSA-2048 / SHA-256)"
else
    echo "[FAIL] (assinatura vazia ou falhou)"
    exit 1
fi

echo -n "Testando validação da assinatura oficial contra chave pública correspondente... "
if openssl dgst -sha256 -verify "${TMP_DIR}/test_rsa_pub.pem" -signature "${DIST_DIR}/SHA256SUMS.sig" "${DIST_DIR}/SHA256SUMS" >/dev/null 2>&1; then
    echo "[PASS] (assinatura verificada com sucesso via openssl dgst -sha256 -verify)"
else
    echo "[FAIL] (verificação de assinatura falhou)"
    exit 1
fi

echo -n "Testando rejeição de binário adulterado após a assinatura... "
echo "malicious payload injection" >> "${DIST_DIR}/lsm-control-plane-linux-amd64"
set +e
(
    cd "${DIST_DIR}"
    sha256sum -c SHA256SUMS >/dev/null 2>&1
)
tamper_bin_res=$?
set -e
if [[ "${tamper_bin_res}" -ne 0 ]]; then
    echo "[PASS] (rejeitado imediatamente por falha de hash)"
else
    echo "[FAIL] (adulteração não detectada)"
    exit 1
fi
# Restaura binário
printf '\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x3e\x00' > "${DIST_DIR}/lsm-control-plane-linux-amd64"
echo "Simulated binary payload for lsm-control-plane-linux-amd64 (LSM 1.2.2 release pipeline dry-run)" >> "${DIST_DIR}/lsm-control-plane-linux-amd64"

echo -n "Testando rejeição de manifesto SHA256SUMS adulterado... "
echo "0000000000000000000000000000000000000000000000000000000000000000  fake-binary" >> "${DIST_DIR}/SHA256SUMS"
set +e
openssl dgst -sha256 -verify "${TMP_DIR}/test_rsa_pub.pem" -signature "${DIST_DIR}/SHA256SUMS.sig" "${DIST_DIR}/SHA256SUMS" >/dev/null 2>&1
tamper_manifest_res=$?
set -e
if [[ "${tamper_manifest_res}" -ne 0 ]]; then
    echo "[PASS] (assinatura inválida detectada com sucesso)"
else
    echo "[FAIL] (adulteração do manifesto não detectada pela assinatura)"
    exit 1
fi
# Restaura manifesto
(
    cd "${DIST_DIR}"
    sha256sum "${BINARIES[@]}" > SHA256SUMS
)

echo -n "Testando rejeição de assinatura forjada com chave privada não autorizada... "
sign_with_key "${TMP_DIR}/attacker_priv.pem" "${DIST_DIR}/SHA256SUMS" "${DIST_DIR}/SHA256SUMS.sig"
set +e
openssl dgst -sha256 -verify "${TMP_DIR}/test_rsa_pub.pem" -signature "${DIST_DIR}/SHA256SUMS.sig" "${DIST_DIR}/SHA256SUMS" >/dev/null 2>&1
forged_sig_res=$?
set -e
if [[ "${forged_sig_res}" -ne 0 ]]; then
    echo "[PASS] (assinatura forjada rejeitada contra a chave pública oficial)"
else
    echo "[FAIL] (assinatura forjada aceita incorretamente)"
    exit 1
fi

echo -n "Testando detecção de fingerprint divergente de chave pública... "
ACTUAL_FP=$(sha256sum "${TMP_DIR}/attacker_pub.pem" | awk '{print $1}')
if [[ "${ACTUAL_FP}" != "${TEST_PUB_FINGERPRINT}" ]]; then
    echo "[PASS] (divergência de fingerprint detectada: esperada ${TEST_PUB_FINGERPRINT:0:16}..., obtida ${ACTUAL_FP:0:16}...)"
else
    echo "[FAIL] (fingerprint deveria ser diferente)"
    exit 1
fi

echo "=== Todos os testes do fluxo de release dry-run foram APROVADOS! ==="
