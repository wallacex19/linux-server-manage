# Guia de Instalação de Produção / Production Installation Guide — Linux Server Manager (LSM)

<p align="center">
  <a href="#versão-em-português"><strong>🇧🇷 Português</strong></a> •
  <a href="#english-version"><strong>🇺🇸 English</strong></a>
</p>

---

<a id="versão-em-português"></a>
## 🇧🇷 Versão em Português

Instruções para deploy automatizado e manual do Linux Server Manager no **Ubuntu Server 24.04 LTS** (arquiteturas `amd64` e `arm64`).

---

### 0. Raiz de Confiança e Bootstrap Inicial Seguro

> [!CAUTION]
> **Defesa em Profundidade contra Execução Não Confiável:** Nunca execute comandos como `curl | sudo bash` sem validação prévia. O Linux Server Manager exige que a integridade do código de bootstrap seja confirmada **antes** da elevação para `root`.

#### 0.1 Cenário A: Clone via Git com Tags Assinadas Criptograficamente (Recomendado)
Ao obter o código via repositório Git, valide a assinatura da tag oficial:
```bash
git clone --branch v1.2.2 https://github.com/wallacex19/linux-server-manage.git /opt/lsm
cd /opt/lsm

# Valida a assinatura criptográfica do mantenedor oficial da versão
git tag -v v1.2.2
```
Apenas prossiga para a instalação se o Git confirmar `Good signature`.

#### 0.2 Cenário B: Download de Release Tarball com Verificação Out-of-Band
Ao transferir o pacote tarball da release:
1. Baixe o pacote oficial, o manifesto `SHA256SUMS` e a assinatura `SHA256SUMS.sig`.
2. Confirme o fingerprint SHA-256 da chave pública oficial do CI:
   ```bash
   sha256sum deploy/scripts/certs/ci-release.pub
   # Deve exibir estritamente: 43ff16c386fdf1982f28c1ba18c68ee529a7d58115b5a8647bf3e40669b8fc40
   ```
3. Valide a assinatura digital antes de executar qualquer instalador:
   ```bash
   openssl dgst -sha256 -verify deploy/scripts/certs/ci-release.pub -signature SHA256SUMS.sig SHA256SUMS
   ```
4. Verifique os binários contra o manifesto:
   ```bash
   sha256sum -c SHA256SUMS
   ```

---

### 1. Instalação Automatizada Oficial

O LSM conta com uma suite completa de scripts de instalação idempotentes e modulares em `deploy/scripts/`.

#### 1.1 Instalar o Control Plane (Painel Central + API REST + Frontend)
Execute como root ou com `sudo`:
```bash
sudo bash deploy/scripts/install.sh --role control-plane --domain lsm.exemplo.com
```

O instalador executa automaticamente:
- Validação do SO (Ubuntu 24.04 LTS) e arquitetura (`amd64` / `arm64`).
- Instalação de dependências (`curl`, `tmux`, `sqlite3`, `caddy`).
- Criação de usuário restrito de sistema `lsm-server` e diretório `/var/lib/lsm-control-plane` (permissão estrita `750`).
- Instalação do binário `/usr/local/bin/lsm-control-plane` e frontend estático em `/var/www/lsm-web/dist`.
- Configuração do serviço systemd `lsm-control-plane.service`.
- Configuração do Caddy com TLS automático para o domínio fornecido.
- Verificação de saúde da API local via endpoint `/api/v1/health`.

#### 1.2 Instalar o Server Agent em Nós Remotos
No servidor Linux que deseja administrar:
```bash
sudo bash deploy/scripts/install.sh --role agent --panel-url https://lsm.exemplo.com --token LSM-PAIR-XXXX
```

O instalador do agente realiza:
- Criação de usuário restrito de sistema `lsm-agent` e diretório `/var/lib/lsm-agent` (permissão estrita `750`).
- Geração da identidade criptográfica individual **Ed25519** em `/var/lib/lsm-agent/identity.key` (permissão estrita `600`).
- Pareamento seguro com o painel central utilizando o token de uso único.
- Instalação e inicialização do serviço systemd `lsm-agent.service`.
- Estabelecimento do túnel reverso via WebSocket criptografado (`wss://`).

---

### 2. Diagnóstico e Verificação Pós-Instalação

Para verificar o estado dos serviços, permissões e endpoints de saúde a qualquer momento:
```bash
sudo bash deploy/scripts/install.sh --mode check
```

---

### 3. Modo de Simulação (Dry-Run)

Para simular o que será executado antes de aplicar qualquer alteração no sistema operacional:
```bash
sudo bash deploy/scripts/install.sh --role control-plane --dry-run
```

---

### 4. Atualização e Desinstalação

- **Atualização sem perda de dados:**
  ```bash
  sudo bash deploy/scripts/install.sh --role control-plane --mode upgrade
  ```
- **Desinstalação segura:**
  ```bash
  sudo bash deploy/scripts/install.sh --role control-plane --mode uninstall
  ```
  *(Por padrão, bancos de dados SQLite e chaves criptográficas são preservados. Use `--purge` apenas se desejar apagar todos os dados permanentemente).*

---

### 5. Política de Binários de Produção e Governança de Toolchain Go

Para garantir integridade criptográfica e conformidade de segurança no LSM 1.2.2:

1. **Preferência e Validação de Binários Pré-compilados:**
   - Servidores de produção não devem possuir ferramentas de desenvolvimento nem compiladores Go instalados.
   - Forneça os executáveis compilados pelo pipeline oficial de CI em `services/control-plane/lsm-control-plane` e `services/server-agent/lsm-agent`.
   - O instalador executa compulsoriamente a função `verify_precompiled_binary`:
     - **Formato:** Validação estrita de executável ELF 64-bit Linux.
     - **Arquitetura:** Correspondência exata com a arquitetura do host (`amd64` ou `arm64`).
     - **Toolchain de Origem:** Verificação da assinatura do compilador embutido no binário; binários compilados com Go desatualizado (ex: Go 1.23.0 local) são **rejeitados** para produção.
     - **Integridade Criptográfica:** Validação de hash SHA-256 contra o manifesto assinado (`.sha256` ou `SHA256SUMS`). Binários com hash divergente abortam a instalação imediatamente.

2. **Compilação a partir do Código-Fonte (Se aplicável):**
   - Caso opte por compilar diretamente no host, a função `validate_build_toolchain` exige **Go 1.26.10+** ou **Go 1.27.3+** (versões homologadas no CI com patches de segurança contra vulnerabilidades da biblioteca padrão).
   - O instalador compila com `GOTOOLCHAIN=local` e `CGO_ENABLED=0`, impedindo que o Go tente baixar automaticamente versões não auditadas da internet.
   - Compiladores legados (< 1.26) são bloqueados por padrão. Em ambientes de desenvolvimento/teste, o build pode ser forçado com:
     ```bash
     export LSM_ALLOW_UNSUPPORTED_COMPILER=true
     sudo -E bash deploy/scripts/install.sh --role control-plane ...
     ```

---

<a id="english-version"></a>
## 🇺🇸 English Version

Automated and manual production deployment instructions for Linux Server Manager on **Ubuntu Server 24.04 LTS** (`amd64` and `arm64` architectures).

---

### 0. Root of Trust & Secure Initial Bootstrap

> [!CAUTION]
> **Defense in Depth Against Untrusted Execution:** Never pipe untrusted code into a privileged shell (e.g., `curl | sudo bash`). Linux Server Manager enforces cryptographic verification of bootstrap code **before** elevating privileges to `root`.

#### 0.1 Scenario A: Git Clone with Cryptographically Signed Tags (Recommended)
When cloning the repository via Git, verify the official maintainer tag signature:
```bash
git clone --branch v1.2.2 https://github.com/wallacex19/linux-server-manage.git /opt/lsm
cd /opt/lsm

# Verify cryptographic signature of the official release tag
git tag -v v1.2.2
```
Only proceed if Git confirms `Good signature`.

#### 0.2 Scenario B: Release Tarball Download with Out-of-Band Verification
When downloading release artifacts directly:
1. Download the release tarball, the `SHA256SUMS` manifest, and the `SHA256SUMS.sig` signature.
2. Confirm the SHA-256 fingerprint of the official CI release public key:
   ```bash
   sha256sum deploy/scripts/certs/ci-release.pub
   # Must strictly match: 43ff16c386fdf1982f28c1ba18c68ee529a7d58115b5a8647bf3e40669b8fc40
   ```
3. Validate digital signature before executing any installer:
   ```bash
   openssl dgst -sha256 -verify deploy/scripts/certs/ci-release.pub -signature SHA256SUMS.sig SHA256SUMS
   ```
4. Verify precompiled binaries against manifest:
   ```bash
   sha256sum -c SHA256SUMS
   ```

---

### 1. Official Automated Production Installation

LSM includes an idempotent, modular Bash installation suite located in `deploy/scripts/`.

#### 1.1 Install Control Plane (Web Dashboard + REST API + Reverse Proxy)
Run as `root` or with `sudo`:
```bash
sudo bash deploy/scripts/install.sh --role control-plane --domain lsm.example.com
```

The installer automatically handles:
- Host OS validation (Ubuntu 24.04 LTS) and architecture (`amd64` / `arm64`).
- Base dependency installation (`curl`, `tmux`, `sqlite3`, `caddy`).
- Dedicated system account creation `lsm-server` and directory `/var/lib/lsm-control-plane` (strict `750` permissions).
- Binary installation to `/usr/local/bin/lsm-control-plane` and static SPA build to `/var/www/lsm-web/dist`.
- Systemd service configuration for `lsm-control-plane.service`.
- Caddy reverse proxy setup with automated TLS certificate management.
- Local API health check verification via `/api/v1/health`.

#### 1.2 Install Server Agent on Remote Managed Nodes
On the target Linux node you want to monitor and manage:
```bash
sudo bash deploy/scripts/install.sh --role agent --panel-url https://lsm.example.com --token LSM-PAIR-XXXX
```

The agent installer performs:
- Dedicated system account creation `lsm-agent` and directory `/var/lib/lsm-agent` (strict `750` permissions).
- Unique **Ed25519** asymmetric key pair generation at `/var/lib/lsm-agent/identity.key` (strict `600` permissions).
- Atomic node registration and pairing with the Control Plane using the single-use token.
- Installation and activation of `lsm-agent.service`.
- Establishment of the outbound encrypted reverse WebSocket tunnel (`wss://`).

---

### 2. Post-Installation Diagnostic Check

To audit systemd units, directory permissions, and API health endpoints at any time:
```bash
sudo bash deploy/scripts/install.sh --mode check
```

---

### 3. Dry-Run Simulation Mode

To review planned actions without making changes to the host:
```bash
sudo bash deploy/scripts/install.sh --role control-plane --dry-run
```

---

### 4. Upgrades and Safe Uninstallation

- **In-place Upgrade (Preserves Databases & Keys):**
  ```bash
  sudo bash deploy/scripts/install.sh --role control-plane --mode upgrade
  ```
- **Safe Uninstallation:**
  ```bash
  sudo bash deploy/scripts/install.sh --role control-plane --mode uninstall
  ```
  *(Databases and keys are kept intact by default. Pass `--purge` only if you wish to remove all data permanently).*

---

### 5. Production Binary Policy & Go Toolchain Governance

To guarantee cryptographic integrity and supply chain security in LSM 1.2.2:

1. **Precompiled Binaries Preferred:**
   - Production servers must not host compilers or development SDKs.
   - Provide binaries built and signed by the official CI pipeline in `services/control-plane/lsm-control-plane` and `services/server-agent/lsm-agent`.
   - The installer automatically enforces `verify_precompiled_binary`:
     - **Format:** Strict 64-bit Linux ELF binary check.
     - **Architecture:** Must match host architecture (`amd64` or `arm64`).
     - **Toolchain Provenance:** Compiler signature validation; outdated Go builds (e.g. Go 1.23.0 local) are **rejected** for production.
     - **Cryptographic Hash:** SHA-256 validation against signed manifest (`.sha256` or `SHA256SUMS`). Mismatched binaries trigger an immediate abort.

2. **Compiling from Source (Development / Custom Environments):**
   - If compiling directly on the host, `validate_build_toolchain` requires **Go 1.26.10+** or **Go 1.27.3+** (CI-validated toolchains containing standard library security fixes).
   - Compilation runs with `GOTOOLCHAIN=local` and `CGO_ENABLED=0`, preventing unverified compiler auto-downloads.
   - Legacy compilers (< 1.26) are blocked by default. In development environments, this check can be overridden with:
     ```bash
     export LSM_ALLOW_UNSUPPORTED_COMPILER=true
     sudo -E bash deploy/scripts/install.sh --role control-plane ...
     ```
