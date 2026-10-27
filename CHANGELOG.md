# Changelog — Linux Server Manager (LSM)

Todas as alterações notáveis neste projeto serão documentadas neste arquivo.
O formato é baseado no [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/) e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [1.2.2] - 2026-10-25

### Adicionado e Corrigido (Fechamento de Segurança e Produção)
- **Encerramento de Processos via Menor Privilégio Estrito (Opção A) e `pidfd`:**
  - Desativação completa do subcomando privilegiado `kill` em `deploy/scripts/lsm-privileged-helper.sh` e revogação total da permissão sudoers correspondente em `deploy/scripts/install-agent.sh`.
  - Sinalização restrita aos processos sob o UID de `lsm-agent` e processos filhos gerenciados, despachada atomicamente via `pidfd` (`SYS_PIDFD_OPEN` [434] e `SYS_PIDFD_SEND_SIGNAL` [424]) para eliminar condições de corrida de reaproveitamento de PID.
  - Bloqueio imediato pelo kernel (`EPERM`) de qualquer tentativa de sinalizar processos de outros operadores ou root/PID 1.
  - Interface web (`apps/web/src/views/ProcessesView.tsx`) aprimorada com banner de aviso "Política de Menor Privilégio Ativa" e tratamento fiel de erros, sem induzir a falsos sucessos.
- **Isolamento Estrito de Rede e Defesa em Profundidade do Control Plane:**
  - Configuração do listener HTTP padrão em loopback (`127.0.0.1` e suporte unificado a `[::1]` com `net.JoinHostPort`).
  - Em ambiente de produção (`LSM_ENV=production`), bloqueio compulsório de inicialização escutando em `0.0.0.0` ou `::`, salvo se explicitamente autorizado por `LSM_ALLOW_EXTERNAL_LISTEN=true`.
  - `NewTrustedProxyMiddleware`: suporte a proxies locais (loopback) e remotos (`LSM_TRUSTED_PROXIES` com IPs e CIDRs). Bloqueio de conexões diretas externas com HTTP 403 Forbidden. Tráfego de proxies confiáveis repassa `X-Forwarded-For` para auditoria, enquanto clientes não confiáveis têm cabeçalhos ignorados. Desacoplamento estrito de autenticação e permissões em relação ao IP recebido.
  - Implementação de 7 testes de isolamento de rede em `services/control-plane/internal/api/network_isolation_test.go` (100% aprovados).
- **Governança de Toolchain no Instalador e Dependência Chi 5.3.2:**
  - Função `validate_build_toolchain` em `deploy/scripts/lib/dependencies.sh`: bloqueio de compiladores Go < 1.23 na instalação a partir de fonte e preferência por binários pré-compilados verificados em CI.
  - Configuração do pipeline GitHub Actions (`.github/workflows/ci.yml`) com versões fixadas e reproduzíveis do Go (`1.26.10` e `1.27.3`), com etapa dedicada de `govulncheck`.
  - Atualização de `github.com/go-chi/chi/v5` para `v5.3.2` (mitigação completa de GO-2026-5777, GO-2026-5775 e GO-2026-5774).
- **Resolução de Alertas do `npm audit` e Atualização de Dependências:**
  - Resolução das 2 vulnerabilidades críticas em `tinypool` (GHSA-5gmw-xhrv-c9v3 e GHSA-85c8-ppgw-ccpr) via override `tinypool ^2.1.2`.
  - Resolução da vulnerabilidade em `postcss-selector-parser`.
  - `npm audit --omit=dev`: 0 vulnerabilidades em dependências de produção.
  - Tolerância zero para falhas em produção no CI (`npm audit --omit=dev --audit-level=high`) e auditoria transparente para devDependencies.
- **Conformidade Total em Ferramentas de Segurança (Windows e Linux):**
  - `gosec`: 0 issues em ambos os módulos (`services/control-plane` e `services/server-agent`), tanto para alvos Windows quanto para alvos Linux (`GOOS=linux`), cobrindo `safepath_linux.go`, `collector_linux.go` e `processes_linux.go`.
  - `gitleaks`: verificação completa do histórico de commits (`gitleaks detect -v`), alcançando 0 segredos vazados. Chaves mock de testes adicionadas a `.gitleaksignore`.
  - `shellcheck`: resolução completa (`-x -P deploy/scripts/lib`) em todos os 13 scripts shell, alcançando 0 erros e 0 alertas.
- **Ambiente de Produção e Política Estrita de TLS:**
  - Implementado fail-safe de ambiente no Control Plane e no Server Agent: se `LSM_ENV` não for definido ou estiver vazio, o sistema assume automaticamente o modo `"production"`.
  - Rejeição estrita de esquemas inseguros (`http://`, `ws://`) em produção.
  - No modo `development`, esquemas inseguros são autorizados exclusivamente sob endereços loopback (`localhost`, `127.0.0.1`, `::1`); qualquer tentativa de usar HTTP/WS para hosts remotos é rejeitada na inicialização.
  - Rejeição explícita de credenciais embutidas em URLs (`http://user:pass@host`).
  - Configuração do cliente WebSocket do agente com `MinVersion: tls.VersionTLS12` e `InsecureSkipVerify: false`.
- **Evolução da Toolchain e Pipeline de Integração Contínua (CI):**
  - Alinhamento com a política oficial de suporte do Go (suporte às duas versões mais recentes: 1.23 e 1.24).
  - Configuração de matriz no GitHub Actions (`.github/workflows/ci.yml`) para testes automáticos em `1.23.x` e `1.24.x` no Ubuntu 24.04 LTS, mantendo `go.mod` em `1.23.0` para máxima compatibilidade.
  - Adição de linter ShellCheck com resolução de dependências (`-x`) e auditoria `npm audit --audit-level=high`.
- **Auditoria e Execução de Ferramentas Adicionais de Segurança:**
  - Varreduras completas executadas no workspace: `govulncheck` (v1.1.3), `gosec` (v2.21.4), `staticcheck` (v0.5.1), `gitleaks` (v8.18.2), `shellcheck` (v0.11.0) e `npm audit`.
  - Correção de alertas de permissões (`0750` em `safepath_fallback.go`), TLS MinVersion 1.2 no dialer WebSocket e formatação de mensagens de erro.
- **Validação de Dry-Run e Não-Mutabilidade dos Instaladores:**
  - Execução validada de `setup-host.sh --dry-run` e `install.sh --dry-run` para papéis de `control-plane` e `agent`.
  - Comprovada a ausência total de mutações em arquivos locais, diretórios do sistema e permissões durante simulações.
- **Autorização Estrita para Logs e Ações de Serviços Systemd (Prioridade 1):**
  - Adicionado middleware `RequireAdmin` nas rotas `/api/v1/system/services/action` e `/api/v1/system/services/logs` (`services/control-plane/internal/api/router.go`).
  - Verificação explícita de `user.IsAdmin` tanto no Control Plane local quanto antes do despacho RPC para nós remotos (`systemmgr_handler.go`, `manager.go`).
  - No Server Agent (`services/server-agent/internal/transport/client.go`), validação dupla de privilégio: a ação `system.service_logs` exige `user.IsAdmin`, rejeitando operadores não-administradores com HTTP 403.
  - Sanitização de nomes de serviço via expressão regular canônica `^[a-zA-Z0-9_@.-]+\.service$` (`services.go`, `systemmgr_handler.go`), rejeitando metacaracteres e injeção de comandos.
  - Limitação estrita do parâmetro `lines` entre 1 e 500 (com teto máximo de 512 KB no buffer de saída).
  - Remoção de conteúdo de logs em traces de auditoria e mensagens de erro no servidor.
- **Pareamento Criptográfico Ed25519 Obrigatório e Imutabilidade de Identidade (Prioridade 2):**
  - Tornada a assinatura Ed25519 estritamente obrigatória no payload de pareamento (`nodes/model.go`, `service.go`).
  - Verificação canônica de assinatura com formato vinculado ao Node ID e Pairing Token (`LSM-PAIR-v1:<nodeID>:<token>`).
  - Rejeição de assinaturas malformadas, comprimentos divergentes de 64 bytes e chaves públicas divergentes de 32 bytes (`ValidateEd25519PublicKey`).
  - Verificação de imutabilidade de identidade: nós já pareados não podem ter suas chaves públicas substituídas silenciosamente.
  - Consumo atômico do token de uso único em transação com invalidação imediata para evitar ataques de repetição.
- **Eliminação de Vetor de Impersonação Arbitrária e Proteção de Processos (Prioridade 3):**
  - Removido completamente o comando `kill-process` do Privileged Helper (`deploy/scripts/lsm-privileged-helper.sh`) e das regras de sudoers (`/etc/sudoers.d/lsm-agent`).
  - Encerramento de processos (`services/server-agent/internal/system/processes.go`) agora opera sob permissões nativas DAC do Linux com o usuário `lsm-agent`, impedindo o envio de sinais para PID <= 1 e para processos pertencentes ao UID 0 (root) por usuários comuns.
- **Proteção Robusta Anti-TOCTOU no Gerenciador de Arquivos (`internal/safepath`) (Prioridade 4):**
  - Criação da biblioteca `services/server-agent/internal/safepath` com implementação nativa Linux (`safepath_linux.go`) utilizando syscall `openat2` com flags do kernel `RESOLVE_BENEATH | RESOLVE_NO_MAGICLINKS` e operações relativas a descritores de diretório (`unlinkat`, `renameat`, `mkdirat`).
  - Fallback seguro para ambiente de desenvolvimento Windows (`safepath_fallback.go`).
  - Rejeição imediata de arquivos especiais (FIFOs, sockets UNIX, nós de dispositivo de bloco/caractere).
  - Suíte de testes unitários e teste de concorrência multithread demonstrando resistência a condições de corrida TOCTOU.
- **Obrigatoriedade de HTTPS/WSS em Produção e Remoção de Token via Query String (Prioridade 5):**
  - Validação de ambiente (`LSM_ENV=production`): exige prefixos `https://` e `wss://` tanto no Control Plane (`services/control-plane/internal/config/config.go`) quanto no Server Agent (`services/server-agent/internal/config/config.go`). Em produção, URLs `http://` ou `ws://` causam falha imediata na inicialização (`Validate()`).
  - Remoção de fallback de autenticação por query string na conexão WebSocket (`internal/nodes/agent_hub.go`); credenciais trafegam estritamente via cabeçalhos HTTP.
- **Instalador de Produção com Versão Canônica Fixa e Dry-Run Real (Prioridades 6 e 7):**
  - `deploy/scripts/setup-host.sh` atualizado para baixar a versão estável especificada (`LSM_VERSION=v1.2.1-fix`), sem clonar silenciosamente o branch instável `main`.
  - Modo `--dry-run` real e estrito em todos os scripts (`setup-host.sh`, `install.sh`, `install-agent.sh`, `install-control-plane.sh`, `uninstall.sh`), garantindo saída limpa sem modificar arquivos ou invocar `visudo`.
  - Tratamento estrito de validação do arquivo de sudoers: caso `visudo -c -f /etc/sudoers.d/lsm-agent` falhe, a instalação remove o arquivo gerado e encerra com código de saída 1.
  - Definição de timeouts de execução no privileged helper (15s para `systemctl`, 10s para `journalctl`) e limite de 500 linhas / 512 KB.
- **Métricas Reais de Telemetria e Processos (Prioridade 8):**
  - Coleta real de utilização de disco via `unix.Statfs` sobre a partição raiz (`/`) e contagem dinâmica de processos ativos via `/proc` no Server Agent (`internal/metrics/collector_linux.go`), eliminando métricas zeradas em `sendHeartbeat`.
  - Obtenção de `VmRSS` real a partir de `/proc/[pid]/status` e linha de comando em `/proc/[pid]/cmdline` (`internal/system/processes.go`).
- **Automação de CI (GitHub Actions):**
  - Adicionado workflow oficial `.github/workflows/ci.yml` para validação em Ubuntu 24.04 (Go 1.23, Node.js 20, cross-compilação `amd64`/`arm64`, testes de concorrência com `-race` e lint de shell scripts).

## [1.2.1] - 2026-10-25

### Adicionado e Integrado
- **Integração Real do Runtime do Server Agent:**
  - Binário `cmd/agent/main.go` totalmente integrado com carregamento de configuração canônica (`agent_config.json`), suporte a flags de linha de comando (`--config`, `--identity-path`, `--export-pubkey`, `--sign-token`) e graceful shutdown via sinais `SIGINT`/`SIGTERM`.
  - Configuração canônica unificada (`internal/config/config.go`) com leitura resiliente de JSON, override por variáveis de ambiente e gravação com permissão estrita `0600`.
  - Unificação do ciclo de vida criptográfico Ed25519 em `internal/identity/identity.go`, gerando identidades e assinaturas criptográficas de prova de posse para pareamento com o Control Plane.
  - Transporte WSS bidirecional (`internal/transport/client.go`) com reconexão resiliente com backoff exponencial (2s a 30s), envio de credenciais estritamente via cabeçalhos HTTP (`X-LSM-Node-ID`, `X-LSM-Agent-Token`, `Authorization: Bearer`), heartbeat contínuo e loop de despacho RPC.
- **Operações Reais de Sistema no Agente (Eliminação de Mocks):**
  - Gerenciamento real de processos (`internal/system/processes.go`) via varredura de `/proc`, suporte a sinais (`SIGTERM`, `SIGKILL`, `SIGHUP`, `SIGINT`), proteção do PID 1 e verificação de propriedade de processos por UID para usuários não-administradores.
  - Gerenciamento de serviços systemd (`internal/system/services.go`) com validação estrita de nomes (`^[a-zA-Z0-9_@.-]+\.service$`), leitura de logs via `journalctl` e execução privilegiada via helper de menor privilégio.
  - Operações de arquivos anti-TOCTOU (`internal/transport/client.go`) utilizando descritores abertos (`os.OpenFile`), verificação de estatísticas de descritor, resolução física de links (`filepath.EvalSymlinks`) e isolamento restrito à `/home/<user>` para contas não-administrativas.
- **Arquitetura de Menor Privilégio (Least-Privilege Helper):**
  - Implementado `deploy/scripts/lsm-privileged-helper.sh` (`/usr/local/libexec/lsm-privileged-helper`) restrito a ações de serviço systemd (`start|stop|restart|reload|status`), sem conceder `NOPASSWD: ALL`.
  - Regra sudoers dedicada em `/etc/sudoers.d/lsm-agent` (modo `0440`).
  - Hardening da unidade `deploy/systemd/lsm-agent.service` com `ProtectSystem=strict`, `ProtectHome=false` e `ReadWritePaths=/var/lib/lsm-agent /var/log/lsm-agent /home` para permitir operações de arquivos do usuário no seu próprio diretório.
- **Correção e Robustez do Instalador Automatizado:**
  - `deploy/scripts/install-agent.sh`: Geração e assinatura Ed25519 automatizada durante o pareamento, instalação do helper e regras sudoers, gravação da configuração canônica `agent_config.json`.
  - `deploy/scripts/setup-host.sh`: Remoção de `|| true`, bootstrap seguro via clone/download em diretório temporário isolado (`/tmp/lsm-bootstrap-XXXXXX`) com limpeza em trap `EXIT`.
  - Remoção de artefatos heredoc (`EOF`) residuais em `deploy/scripts/install.sh`.
- **Validação Criptográfica e Segurança de Comunicação:**
  - Validação estrita de formato e tamanho de chaves públicas Ed25519 (32 bytes) em `services/control-plane/internal/nodes/service.go`.
  - Verificação de assinatura criptográfica autêntica (`ed25519.Verify`) no pareamento de nós para mitigar ataques de personificação.
  - Desconexão ativa e instantânea de WebSocket e rejeição com HTTP 403 Forbidden para nós revogados em `services/control-plane/internal/nodes/agent_hub.go`.
  - URL pública do Control Plane configurável via `LSM_PUBLIC_URL` e `config.Config`.
- **Suíte de Testes de Integração Ponta a Ponta:**
  - `services/control-plane/internal/api/agent_integration_test.go`: Testes cobrindo pareamento com assinatura válida/inválida, proteção contra replay, conexão WSS com headers, envio de heartbeat, despacho RPC bidirecional, desconexão imediata por revogação e isolamento estrito entre múltiplos nós simultâneos.

## [1.2.0] - 2026-10-25

### Adicionado
- **Auditoria de Segurança e Mitigações Críticas:**
  - **Prevenção contra CSWSH (Cross-Site WebSocket Hijacking):** Implementado `CheckWebSocketOrigin` estrito em `terminal/ws_handler.go` e `agent_hub.go`, garantindo que origens cross-site não autorizadas sejam bloqueadas no handshake do WebSocket.
  - **Política Estrita de CORS e Proteção contra CSRF:** Refatorado `corsMiddleware` em `api/router.go` para não refletir origens arbitrárias de atacantes com `Access-Control-Allow-Credentials: true`. Adicionado `csrfProtectionMiddleware` para bloquear requisições mutativas de estado (POST/PUT/DELETE) originadas fora do host ou origens locais aprovadas.
  - **Prevenção contra Symlink Traversal:** Resolução física de symlinks via `filepath.EvalSymlinks` em `files/service.go` e no agente `transport/client.go`.
  - **Sanitização Estrita do SteamCMD contra Injeção de Comandos:** Validação por regex de `AppID` (`^[0-9]+$`), validação de branch `Beta` (`^[a-zA-Z0-9_.-]*$`), sanitização de `InstallPath` contra metacaracteres de shell (`;&|`$><"`) e bloqueio de atualizações concorrentes no mesmo servidor.
  - **Consumo Atômico de Pairing Tokens:** Transação com verificação estrita de concorrência (`RowsAffected == 1`) e validação de chaves públicas em `nodes/service.go`.
  - **Roteamento e Autorização Negativa de Nós Remotos:** Suporte a despacho remoto por `node_id` em `FilesHandler` e `SystemManagerHandler`, retornando HTTP 404 (nó inexistente), HTTP 403 (nó revogado) e HTTP 503 (nó offline) antes de encaminhar comandos RPC.
  - **Instalador Automatizado de Produção Oficial (`deploy/scripts/`):**
  - **Suite Modular e Idempotente:** Conjunto estruturado em `deploy/scripts/` com bibliotecas reutilizáveis (`logging.sh`, `os-detection.sh`, `dependencies.sh`, `validation.sh`, `common.sh`).
  - **Orquestrador Central `install.sh`:** Suporte completo a papéis `--role control-plane` e `--role agent`, e modos `--mode install`, `upgrade`, `uninstall`, `check`, `dry-run`.
  - **Instaladores Dedicados:** `install-control-plane.sh` (usuário `lsm-server`, Caddy, systemd) e `install-agent.sh` (usuário `lsm-agent`, chaves Ed25519, pareamento).
  - **Preservação de Dados e Identidades:** Script de upgrade (`upgrade.sh`) e desinstalação segura (`uninstall.sh`) preservando bancos SQLite e chaves criptográficas por padrão.
  - **Diagnóstico Automático (`check-installation.sh`):** Verificação de integridade de serviços systemd, portas de escuta, auditoria de permissões (750/600) e healthcheck da API REST.

## [1.1.0] - 2026-10-25

### Adicionado
- **Arquitetura Multi-Servidor Centralizada (Control Plane + Server Agent):**
  - **Túneis Reversos via WebSocket:** Conexões iniciadas pelos agentes em direção ao Control Plane (`wss://painel/api/v1/agent/connect`), dispensando abertura de portas públicas nos nós gerenciados e atravessando NATs de forma transparente.
  - **Identidade Criptográfica Ed25519 e Pareamento Seguro:** Geração de chaves assimétricas locais no agente e pareamento por token de uso único (`LSM-PAIR-XXXX`) com TTL de 15 minutos, gerando token permanente de 256 bits.
  - **Motor de Roteamento AgentHub:** Despacho de mensagens RPC com RequestID, correlation IDs e timeouts de 30 segundos, com detecção e prevenção de repetição.
  - **Detecção de Queda e Heartbeat Contínuo:** Amostragem periódica de métricas a cada 10s e marcação automática como offline caso não haja sinal por mais de 30s, mantendo cache com indicador de dados desatualizados.
  - **Validação Estrita de Contas Linux Locais:** `AccountChecker` no agente verifica a existência prévia da conta do operador no servidor Linux remoto (`/etc/passwd`). Caso não exista, recusa a operação com mensagem explícita e sem transmitir senhas pela rede.
  - **Revogação Instantânea de Acesso:** Endpoint `/api/v1/nodes/{id}/revoke` para invalidação imediata de credenciais de agentes comprometidos ou desativados.
  - **Interface Web Multi-Servidor (`apps/web`):** Seletor de servidor, `ServersView.tsx`, dashboard integrado.

## [1.0.0] - 2026-10-25
- Versão inicial com SteamCMD, Terminal tmux, Filesystem e Monitoramento de Processos/Serviços.
