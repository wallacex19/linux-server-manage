# Definição Oficial da Stack Tecnológica — Linux Server Manager (LSM)

Este documento estabelece a stack tecnológica obrigatória e permanente do projeto **Linux Server Manager**, complementando a especificação funcional e a arquitetura multi-servidor distribuída.

---

## 1. Stack Definida e Componentes

### 1.1 Frontend
- **Framework & Linguagem:** React 18+ com TypeScript.
- **Build Tool:** Vite.
- **Estilização & Componentes:** Tailwind CSS, shadcn/ui, Lucide Icons.
- **Gerenciamento de Estado & Requisições:** TanStack Query (React Query v5).
- **Visualização de Dados:** Recharts.
- **Terminal Web:** xterm.js (com addons `addon-fit`, `addon-web-links`).
- **Testes Unitários e Componentes:** Vitest + React Testing Library.
- **Testes Ponta a Ponta (E2E):** Playwright.

### 1.2 Backend Central (Painel & Hub de Agentes)
- **Linguagem & Runtime:** Go (MSRV: 1.23.0; Produção/CI: Go 1.26.10+ / 1.27.3+).
- **Roteamento HTTP:** Chi (`github.com/go-chi/chi/v5` v5.3.2).
- **Contrato de API:** REST documentado via OpenAPI 3.0 / Swagger.
- **Comunicação em Tempo Real:** WebSocket seguro (WSS) com autenticação mTLS / tokens criptográficos.
- **Armazenamento de Dados:** SQLite (driver moderno `modernc.org/sqlite`), com WAL mode ativado para alta concorrência em configurações, inventário de servidores, metadados de contas e logs de auditoria.
- **Migrações de Banco:** Migrações versionadas, atômicas e reproduzíveis (`golang-migrate/migrate` ou runner SQL embutido versionado).

### 1.3 Agente Linux (Nó Remoto)
- **Linguagem & Runtime:** Go (MSRV: 1.23.0; Produção/CI: Go 1.26.10+ / 1.27.3+; compilação estática `CGO_ENABLED=0` para máxima portabilidade Linux).
- **Ciclo de Vida:** Serviço systemd nativo (`lsm-agent.service`).
- **Coleta de Métricas:** Interfaces nativas do Linux (`/proc/stat`, `/proc/meminfo`, `/proc/diskstats`, `/proc/net/dev`, `sysfs`) sem depender de daemons de terceiros para o monitoramento básico.
- **Módulos do Agente:** Integração modular com:
  - Arquivos e permissões (POSIX permissions, streaming de logs, visualização de configs).
  - Processos e serviços (`systemctl` / cgroups / kill signals sob menor privilégio).
  - Terminais persistentes (integração direta com `tmux`, suporte opcional a `screen`).
  - SteamCMD (gerenciamento dedicado a servidores de jogos dedicados).
- **Identidade Criptográfica:** Par de chaves assimétricas exclusivas (Ed25519) geradas no primeiro boot do agente, associadas ao ID do nó no painel central.
- **Segurança da Conexão:** Comunicação de saída via WebSocket TLS bidirecional com heartbeat e verificação de assinatura por mensagem.

---

## 1.5 Política de Versões e Toolchain Go (LSM 1.2.2)

O LSM adota uma separação rigorosa entre versão mínima de linguagem, matriz de teste em CI e compilador de produção:

1. **Versão Mínima de Linguagem (MSRV - `go.mod`):**
   - **`go 1.23.0`** em ambos os módulos (`services/control-plane/go.mod` e `services/server-agent/go.mod`).
   - Define a compatibilidade de sintaxe da linguagem (generics, loopvar, min/max) e dependências.
   - Preserva o ciclo de desenvolvimento local e suporte a distribuições sem forçar o mecanismo de auto-download ou quebrar ambientes offline.
   - Nenhuma funcionalidade de sintaxe requer exclusivamente versões superiores a 1.23.

2. **Versões Testadas no CI (`.github/workflows/ci.yml`):**
   - **Go 1.26.10** e **Go 1.27.3** no Ubuntu 24.04 LTS.
   - Ambas as etapas de pipeline executam com **`GOTOOLCHAIN=local`** para garantir que o Go utilize estritamente a versão provisionada pelo runner, sem auto-switch ou download implícito de toolchains.
   - Executa `go vet`, testes concorrentes com `-race`, `govulncheck` e cross-compilação cruzada para `linux/amd64` e `linux/arm64`.

3. **Toolchain Recomendada para Builds de Produção:**
   - **Binários Pré-compilados (Recomendado):** Servidores de produção devem receber binários já compilados e verificados pelo pipeline de CI (`services/control-plane/lsm-control-plane` e `services/server-agent/lsm-agent`), eliminando a presença de compiladores em hosts produtivos.
   - **Compilação Local (Restrita):** Se for necessário compilar no host, a função [`validate_build_toolchain`](deploy/scripts/lib/dependencies.sh) impõe que o compilador pertença às versões estáveis com patches de segurança aplicados (**Go 1.26.10+** ou **Go 1.27.3+**). Versões legadas ou com CVEs conhecidos na standard library são **bloqueadas** com código 2 (salvo override explícito com `LSM_ALLOW_UNSUPPORTED_COMPILER=true` para desenvolvimento).
   - Todo build local no script de deploy aplica `GOTOOLCHAIN=local` para impedir que o compilador baixe versões não auditadas da internet.

### 1.4 Sistema Operacional e Infraestrutura
- **Ambiente Alvo Prioritário:** Ubuntu Server 24.04 LTS ou superior.
- **Autenticação:** PAM (Pluggable Authentication Modules) no host Linux para login dos administradores/operadores.
- **Sessões Persistentes:** `tmux` como implementação preferencial; `screen` como fallback/suporte opcional.
- **Proxy Reverso:** Caddy (com TLS automático e terminação segura de WebSocket).
- **Controle de Versão:** Git.
- **Deploy de Produção:** Binários Go autônomos e serviços systemd (sem overhead obrigatório de containers).
- **Docker:** Opcional (não mandatório, usado apenas caso o operador prefira isolar certos game servers).

---

## 2. Regras Rígidas de Implementação

1. **Não trocar a stack definida** sem uma justificativa técnica documentada e aprovada.
2. **Seleção de versões estáveis:** dependências fixadas no início de cada fase.
3. **Sem upgrades indiscriminados:** atualizações ocorrem em etapas dedicadas com testes de regressão.
4. **Separação estrita de camadas:** frontend, backend central e agente Linux residem em diretórios e módulos desacoplados. Não há importação cruzada de lógicas operacionais internas.
5. **Sem banco de senhas paralelo:** a autenticação deriva das contas e políticas do sistema Linux (via PAM / chaves SSH / tokens de sessão de curta duração com PAM).
6. **Princípio do menor privilégio (Não root geral):**
   - O painel central e o agente executam sob usuários dedicados do sistema (`lsm-server` e `lsm-agent`).
   - Operações que demandam privilégios específicos utilizam helpers granulares via `sudo` com comandos restritos no `/etc/sudoers.d/lsm`, nunca shell root irrestrito.
7. **Sem execução arbitrária de comandos privilegiados:** comandos operacionais são parametrizados e validados contra uma whitelist estrita no agente.
8. **Validação de autorização em duas etapas:** autorização validada no painel central e revalidada no agente local antes de qualquer execução.
9. **Cobertura de testes obrigatória:** testes para autenticação, isolamento multi-tenant/multi-servidor, permissões de arquivos, sessões de terminal e auditoria.
10. **Evidência de execução:** nenhuma funcionalidade é marcada como concluída sem log/evidência de testes aprovados.
11. **Evolução incremental:** sem refatorações destrutivas do zero.
