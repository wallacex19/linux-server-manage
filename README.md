# Linux Server Manager (LSM)

<p align="center">
  <a href="#versão-em-português"><strong>🇧🇷 Português</strong></a> •
  <a href="#english-version"><strong>🇺🇸 English</strong></a>
</p>

---

<a id="versão-em-português"></a>
## 🇧🇷 Versão em Português

> Painel centralizado e agente distribuído para administração de infraestrutura Linux e servidores dedicados de jogos (SteamCMD, tmux, systemd).

---

### 🌟 Visão Geral e Recursos Principais

- **Gerenciamento Centralizado Multi-Servidor:** Controle múltiplos servidores Linux remotos a partir de um único painel web através de túneis reversos WebSocket seguros (compatível com NAT, sem portas abertas nos agentes).
- **Autenticação Nativa no Host (PAM):** Login direto com contas de usuário existentes no sistema operacional Linux. **Sem banco de senhas paralelo nem sincronização de credenciais.**
- **Gerenciamento de Arquivos em Sandbox:** Navegador de arquivos web, editor integrado, upload/download, auditoria de permissões POSIX e prevenção ativa contra **Symlink Traversal**.
- **Gestão de Serviços do Sistema (systemd):** Monitoramento, inicialização, parada e reinício de unidades systemd com menor privilégio e permissões granulares.
- **Servidores de Jogos Dedicados & SteamCMD:** Instalação e controle de servidores Valve em um clique (CS2, Valheim, Palworld, Rust, TF2, Project Zomboid) em sessões persistentes via `tmux`, com validação de runtime 32-bit e sanitização rigorosa contra injeção de comandos.
- **Telemetria de Recursos em Tempo Real:** Coleta nativa via `/proc` (CPU por núcleo, memória RAM, Swap, disco, tráfego de rede e sensores térmicos) com buffer circular e gráficos em tempo real.
- **Terminal Web Integrado:** Acesso persistente via browser (xterm.js) com isolamento estrito de sockets `tmux` por usuário (`tmux -L lsm-<username>`).
- **Instalador de Produção Automatizado:** Scripts modulares e idempotentes em Bash para Ubuntu Server 24.04 LTS.

---

### 🏗 Arquitetura e Estrutura de Diretórios

O LSM é estruturado como um monorepo modular e desacoplado:

```text
linux-server-manage/
├── apps/
│   └── web/                   # Frontend SPA (React 18 + TypeScript + Vite + Tailwind CSS + shadcn/ui)
├── services/
│   ├── control-plane/         # Backend central (Go + Chi + SQLite WAL + Hub WSS Reverso + OpenAPI)
│   └── server-agent/          # Agente Linux distribuído (Go + systemd + Ed25519 + /proc)
├── packages/
│   └── api-contracts/         # Especificações OpenAPI 3.0 e contratos de DTOs (openapi.yaml)
├── deploy/
│   ├── systemd/               # Unit files hardened do systemd (lsm-control-plane, lsm-agent)
│   ├── caddy/                 # Configuração de referência do Caddyfile (HTTPS / Proxy Reverso)
│   └── scripts/               # Suite do instalador de produção (install.sh, upgrade.sh, etc.)
│       └── certs/             # Chaves públicas oficiais e raiz de confiança de releases
├── docs/                      # Documentação de arquitetura, segurança, instalação e API
├── migrations/                # Migrações versionadas do banco de dados SQLite
├── tests/                     # Testes de integração, segurança e conformidade
├── TECH_STACK.md              # Especificação oficial permanente da stack tecnológica
├── PROJECT_STATUS.md          # Matriz de progresso e evidências reais de execução
├── CHANGELOG.md               # Histórico versionado de alterações
└── README.md                  # Este guia
```

---

### 📋 Pré-requisitos do Sistema

- **Sistema Operacional Homologado:** Ubuntu Server 24.04 LTS (arquiteturas `amd64` e `arm64`).
- **Governança de Toolchain Go:**
  - **MSRV (Mínimo de Linguagem):** Go 1.23.0 definido em `go.mod` para compatibilidade sintática.
  - **Toolchain de CI e Produção:** Go 1.26.10+ ou Go 1.27.3+ com `GOTOOLCHAIN=local` para patches de segurança da biblioteca padrão.
  - Servidores de produção devem preferencialmente executar os **binários pré-compilados** assinados no CI.
- **Node.js & npm:** Node.js 20+ e npm 10+ (apenas para desenvolvimento e compilação do frontend SPA).
- **Proxy Reverso:** Caddy v2 (configurado automaticamente pelo instalador oficial para terminação TLS).

---

### 🚀 Comandos de Desenvolvimento Local e Testes

#### 1. Control Plane (`services/control-plane`)
```bash
cd services/control-plane

# Executar localmente (inicia em http://localhost:8080)
go run ./cmd/server

# Executar suite completa de testes
go test -v -count=1 ./...
```

#### 2. Server Agent (`services/server-agent`)
```bash
cd services/server-agent

# Executar localmente
go run ./cmd/agent

# Executar suite de testes com detecção de concorrência (-race)
go test -v -count=1 -race ./...
```

#### 3. Web App (`apps/web`)
```bash
cd apps/web

# Instalar dependências
npm install

# Servidor de desenvolvimento Vite (acessível em http://localhost:3000)
npm run dev

# Testes unitários e de componentes
npm run test

# Compilação de produção
npm run build
```

---

### 📦 Instalação Automatizada de Produção

Consulte o [Guia Detalhado de Instalação](docs/installation.md).

```bash
# 1. Instalar o Painel Central (Control Plane) com Caddy:
sudo bash deploy/scripts/install.sh --role control-plane --domain lsm.exemplo.com

# 2. Instalar o Agente em nós remotos e parear com o painel central:
sudo bash deploy/scripts/install.sh --role agent --panel-url https://lsm.exemplo.com --token LSM-PAIR-XXXX

# 3. Diagnóstico completo de integridade dos serviços:
sudo bash deploy/scripts/install.sh --mode check

# 4. Atualização segura preservando banco de dados e chaves:
sudo bash deploy/scripts/install.sh --role control-plane --mode upgrade

# 5. Simulação de execução (dry-run):
sudo bash deploy/scripts/install.sh --role control-plane --dry-run
```

---

### ⚙️ Referência de Variáveis de Ambiente

> [!NOTE]
> Os valores padrão são seguros para implantação. Não registre segredos diretamente em arquivos compartilhados.

#### Control Plane (`services/control-plane`)
| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `LSM_HOST` | `127.0.0.1` | Endereço IP de escuta (em produção, restrito a loopback). |
| `LSM_PORT` | `8080` | Porta TCP de escuta do servidor HTTP. |
| `LSM_DB_PATH` | `lsm_panel.db` | Caminho do arquivo SQLite com suporte a WAL. |
| `LSM_ENV` | `production` | Ambiente operacional (`production`, `development`, `testing`). |
| `LSM_PUBLIC_URL` | *Obrigatório em prod* | URL pública canônica HTTPS utilizada para CORS e WebSockets. |
| `LSM_ALLOW_EXTERNAL_LISTEN` | `false` | Se `true`, permite escutar em interfaces não-loopback em produção. |
| `LSM_TRUSTED_PROXIES` | *Vazio* | Lista separada por vírgulas de IPs/CIDRs de proxies upstream confiáveis. |

#### Server Agent (`services/server-agent`)
| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `LSM_CONFIG_PATH` | `/var/lib/lsm-agent/agent_config.json` | Caminho para o arquivo JSON de configuração persistente. |
| `LSM_ENV` | `production` | Ambiente operacional do agente. |
| `LSM_PANEL_URL` | *Obrigatório em prod* | URL pública HTTPS do Control Plane central. |
| `LSM_NODE_ID` | *Gerado no pareamento* | Identificador único do nó registrado no painel. |
| `LSM_AGENT_TOKEN` | *Token de nó* | Token criptográfico de autenticação do agente. |
| `LSM_IDENTITY_PATH` | `/var/lib/lsm-agent/identity.key` | Caminho da chave privada assimétrica Ed25519 (permissão `600`). |
| `LSM_HOSTNAME` | Hostname do sistema | Sobrescrita manual do hostname informado ao painel. |
| `LSM_POLL_INTERVAL` | `5` | Intervalo em segundos de coleta e heartbeat de métricas. |
| `LSM_PRIVILEGED_HELPER`| `/usr/local/libexec/lsm-privileged-helper` | Caminho do helper de menor privilégio para tarefas autorizadas. |

---

### 🔒 Modelo de Segurança e Menor Privilégio

- **Sem Execução como Root Geral:** Nem o Control Plane nem o Agent executam como root. São criadas contas de sistema dedicadas `lsm-server` e `lsm-agent` com shell `/usr/sbin/nologin`.
- **Autenticação Direta via PAM:** O LSM não armazena nem gerencia senhas. O login valida diretamente contra o PAM do Linux no host.
- **Defesa Web Reforçada:** Proteção contra CSWSH com validação estrita de origem (`CheckWebSocketOrigin`), CORS desprovido de reflexão arbitrária e proteção contra CSRF em rotas mutativas.
- **Prevenção Física contra Path & Symlink Traversal:** Resolução física de caminhos via `filepath.EvalSymlinks`, impedindo que symlinks maliciosos escapem do diretório sandboxed do usuário.
- **Sanitização de Comandos SteamCMD:** Validação por expressão regular rigorosa para AppIDs numéricos (`^[0-9]+$`) e nomes de branches beta, impedindo injeção em sessões `tmux`.
- **Identidade Criptográfica Ed25519:** Cada nó possui sua própria chave Ed25519 gerada localmente com permissão estrita `600`, utilizada para assinar o handshake de conexão.
- **Auditoria Imutável:** Todas as ações administrativas são registradas em logs de auditoria no SQLite, sem captura de dados sensíveis ou senhas.

---

### ⚠️ Limitações Conhecidas e Diagnóstico

- **Compatibilidade de SO:** Homologado prioritariamente no Ubuntu Server 24.04 LTS. Outras distribuições baseadas em systemd podem ser utilizadas, mas exigem validação manual dos módulos PAM.
- **SteamCMD em Sistemas 64-bit:** Requer as bibliotecas de compatibilidade 32-bit (`lib32gcc-s1`, `lib32stdc++6`), instaladas automaticamente pelo `install.sh`.
- **Restrição de Loopback em Produção:** Em `LSM_ENV=production`, o backend recusa escutar em `0.0.0.0` a menos que `LSM_ALLOW_EXTERNAL_LISTEN=true` seja expressamente configurado.
- **Diagnóstico Rápido:**
  ```bash
  # Verificar status dos serviços
  sudo systemctl status lsm-control-plane
  sudo systemctl status lsm-agent

  # Inspecionar logs em tempo real
  sudo journalctl -u lsm-control-plane -f
  sudo journalctl -u lsm-agent -f

  # Auditoria pós-instalação
  sudo bash deploy/scripts/install.sh --mode check
  ```

---

<a id="english-version"></a>
## 🇺🇸 English Version

> Centralized web management panel and distributed server agent for Linux infrastructure and dedicated game servers (SteamCMD, tmux, systemd).

---

### 🌟 Overview & Key Features

- **Centralized Multi-Server Management:** Manage multiple remote Linux instances through a single web dashboard using secure reverse WebSocket tunnels (NAT-friendly, zero open inbound ports on agents).
- **Native Host Authentication (PAM):** Users log in directly with their existing host Linux accounts. **No duplicate password databases or synced credentials.**
- **Sandboxed File Manager:** Web-based file explorer, integrated text editor, uploads/downloads, POSIX permission audits, and proactive **Symlink Traversal** defense.
- **Systemd Service Management:** Inspect, start, stop, and restart systemd units with fine-grained, least-privilege policies.
- **Dedicated Game Servers & SteamCMD:** One-click deployment and orchestration of Valve game servers (CS2, Valheim, Palworld, Rust, TF2, Project Zomboid) inside persistent `tmux` sessions, featuring 32-bit runtime validation and strict command injection guards.
- **Real-Time Resource Telemetry:** Native kernel metric gathering via `/proc` (per-core CPU, RAM, Swap, Disk I/O, Network, and thermal sensors) with circular buffers and live charts.
- **Integrated Browser Terminal:** Web-based xterm.js terminal with session persistence via isolated per-user `tmux` sockets (`tmux -L lsm-<username>`).
- **Automated Production Installer:** Idempotent, modular Bash installation suite for Ubuntu Server 24.04 LTS.

---

### 🏗 Architecture & Directory Structure

LSM is architected as a decoupled, modular monorepo:

```text
linux-server-manage/
├── apps/
│   └── web/                   # Frontend SPA (React 18 + TypeScript + Vite + Tailwind CSS + shadcn/ui)
├── services/
│   ├── control-plane/         # Central Backend (Go + Chi + SQLite WAL + Reverse WSS Hub + OpenAPI)
│   └── server-agent/          # Distributed Linux Agent (Go + systemd + Ed25519 + /proc)
├── packages/
│   └── api-contracts/         # OpenAPI 3.0 specs and DTO contracts (openapi.yaml)
├── deploy/
│   ├── systemd/               # Hardened systemd unit files (lsm-control-plane, lsm-agent)
│   ├── caddy/                 # Reference Caddyfile configuration (Automatic HTTPS / Reverse Proxy)
│   └── scripts/               # Production installer suite (install.sh, upgrade.sh, etc.)
│       └── certs/             # Official public keys and release trust root
├── docs/                      # Technical documentation (architecture, security, installation, API)
├── migrations/                # Versioned SQLite database schema migrations
├── tests/                     # Integration, security, and compliance test suites
├── TECH_STACK.md              # Official mandatory technology stack definition
├── PROJECT_STATUS.md          # Progress tracking matrix and execution evidence
├── CHANGELOG.md               # Versioned changelog
└── README.md                  # This guide
```

---

### 📋 System Prerequisites

- **Certified Operating System:** Ubuntu Server 24.04 LTS (`amd64` and `arm64` architectures).
- **Go Toolchain Governance:**
  - **MSRV (Minimum Language Version):** Go 1.23.0 in `go.mod` for syntax compatibility.
  - **CI & Production Toolchains:** Go 1.26.10+ or Go 1.27.3+ with `GOTOOLCHAIN=local` for standard library vulnerability patches.
  - Production servers should preferably run the official CI-compiled and signed **precompiled binaries**.
- **Node.js & npm:** Node.js 20+ and npm 10+ (required only for building or developing the web SPA).
- **Reverse Proxy:** Caddy v2 (automatically set up by the official installer for TLS termination).

---

### 🚀 Local Development & Testing Commands

#### 1. Control Plane (`services/control-plane`)
```bash
cd services/control-plane

# Run locally (starts at http://localhost:8080)
go run ./cmd/server

# Run complete test suite
go test -v -count=1 ./...
```

#### 2. Server Agent (`services/server-agent`)
```bash
cd services/server-agent

# Run locally
go run ./cmd/agent

# Run test suite with race detector enabled (-race)
go test -v -count=1 -race ./...
```

#### 3. Web App (`apps/web`)
```bash
cd apps/web

# Install frontend dependencies
npm install

# Start Vite dev server (accessible at http://localhost:3000)
npm run dev

# Unit and component tests
npm run test

# Production build
npm run build
```

---

### 📦 Automated Production Installation

See the full [Production Installation Guide](docs/installation.md).

```bash
# 1. Install Control Plane with Caddy reverse proxy:
sudo bash deploy/scripts/install.sh --role control-plane --domain lsm.example.com

# 2. Install Server Agent on a remote managed node and auto-pair:
sudo bash deploy/scripts/install.sh --role agent --panel-url https://lsm.example.com --token LSM-PAIR-XXXX

# 3. Comprehensive diagnostic check:
sudo bash deploy/scripts/install.sh --mode check

# 4. Safe in-place upgrade (preserves databases and cryptographic keys):
sudo bash deploy/scripts/install.sh --role control-plane --mode upgrade

# 5. Dry-run simulation mode:
sudo bash deploy/scripts/install.sh --role control-plane --dry-run
```

---

### ⚙️ Environment Variables Reference

> [!NOTE]
> Default values are safe for deployment. Never store plaintext secrets or credentials in tracked files.

#### Control Plane (`services/control-plane`)
| Variable | Default | Description |
| :--- | :--- | :--- |
| `LSM_HOST` | `127.0.0.1` | Bind IP address (in production, restricted to loopback). |
| `LSM_PORT` | `8080` | TCP port for the HTTP server. |
| `LSM_DB_PATH` | `lsm_panel.db` | File path for SQLite database with WAL enabled. |
| `LSM_ENV` | `production` | Operational environment (`production`, `development`, `testing`). |
| `LSM_PUBLIC_URL` | *Required in prod* | Canonical public HTTPS URL used for CORS and WebSockets. |
| `LSM_ALLOW_EXTERNAL_LISTEN` | `false` | When `true`, permits binding to non-loopback interfaces in production. |
| `LSM_TRUSTED_PROXIES` | *Empty* | Comma-separated list of trusted upstream proxy IP or CIDR addresses. |

#### Server Agent (`services/server-agent`)
| Variable | Default | Description |
| :--- | :--- | :--- |
| `LSM_CONFIG_PATH` | `/var/lib/lsm-agent/agent_config.json` | Path to persistent agent JSON configuration file. |
| `LSM_ENV` | `production` | Agent operational environment. |
| `LSM_PANEL_URL` | *Required in prod* | Canonical public HTTPS URL of the central Control Plane. |
| `LSM_NODE_ID` | *Generated on pair* | Unique node ID registered in the panel. |
| `LSM_AGENT_TOKEN` | *Node token* | Node authentication token. |
| `LSM_IDENTITY_PATH` | `/var/lib/lsm-agent/identity.key` | Path to private Ed25519 identity key (`600` permissions). |
| `LSM_HOSTNAME` | System Hostname | Manual override for reported hostname. |
| `LSM_POLL_INTERVAL` | `5` | Metric sampling and heartbeat interval in seconds. |
| `LSM_PRIVILEGED_HELPER`| `/usr/local/libexec/lsm-privileged-helper` | Path to least-privilege helper binary for authorized tasks. |

---

### 🔒 Security Model & Least Privilege Principles

- **Zero Root Execution:** Neither the Control Plane nor Server Agents run as root. Dedicated system accounts (`lsm-server` and `lsm-agent`) are created with `/usr/sbin/nologin` shell.
- **Direct Linux PAM Authentication:** Passwords are verified against the host OS via PAM. Zero storage of passwords or password hashes in application databases.
- **CSWSH, CORS & CSRF Defense:** Strict origin checks on WebSocket handshakes (`CheckWebSocketOrigin`), unreflected CORS credentials, and state-modifying CSRF protection.
- **Physical Symlink Traversal Protection:** All file paths are resolved physically using `filepath.EvalSymlinks`, preventing symlink escapes outside the user's home sandbox.
- **Command Injection Prevention:** Strict regex validation for SteamCMD `AppID` (`^[0-9]+$`) and beta branch names, protecting tmux command execution.
- **Cryptographic Node Identity:** Each node generates its own **Ed25519** key pair locally with strict `600` permissions, signing every connection handshake.
- **Immutable Audit Logging:** All administrative operations are recorded in SQLite audit logs without capturing sensitive tokens or passwords.

---

### ⚠️ Known Limitations & Troubleshooting Pointers

- **Target OS:** Ubuntu Server 24.04 LTS is the primary tier-1 target. Other systemd-based Linux distributions may work, but require verifying PAM module compatibility.
- **SteamCMD on 64-bit Systems:** Requires 32-bit runtime libraries (`lib32gcc-s1`, `lib32stdc++6`), automatically installed by `install.sh`.
- **Production Loopback Binding:** In `LSM_ENV=production`, the backend refuses to bind to `0.0.0.0` unless `LSM_ALLOW_EXTERNAL_LISTEN=true` is explicitly set.
- **Quick Diagnostic Commands:**
  ```bash
  # Check service statuses
  sudo systemctl status lsm-control-plane
  sudo systemctl status lsm-agent

  # Stream live systemd logs
  sudo journalctl -u lsm-control-plane -f
  sudo journalctl -u lsm-agent -f

  # Run installer verification check
  sudo bash deploy/scripts/install.sh --mode check
  ```

---

### 📄 License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
