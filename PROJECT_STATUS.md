# Status do Projeto — Linux Server Manager (LSM)

Última atualização: 25/10/2026 — Versão 1.2.2 (Fechamento de Segurança, Anti-TOCTOU, Menor Privilégio e Validação para Produção)

## 1. Visão Geral do Estado Atual
- **Fase atual:** Versão 1.2.2 — Hardening de Segurança, Mitigações de Concorrência e Pipeline de Produção
- **Status geral:** Aprovado em Testes Automatizados (100% dos testes unitários, testes de segurança, testes de integração remota e builds de produção aprovados)
- **Objetivo do projeto:** Painel web para administração de múltiplos servidores Linux e instâncias de servidores de jogos dedicados a partir de um único painel central, com túneis reversos via WebSocket, pareamento criptográfico Ed25519 estritamente obrigatório, verificação de contas Linux locais e zero banco de senhas paralelo.

---

## 2. Matriz de Componentes e Progresso

| Componente | Localização | Stack | Status | Evidência Real |
| :--- | :--- | :--- | :--- | :--- |
| **Web App** | `apps/web/` | React 18, Vite, TS, Tailwind, Recharts, xterm.js, Vitest | Concluído (v1.2.2) | `npm test -- --run` (PASS - 4/4) & `npm run build` (PASS - dist compilado) & `npm audit --omit=dev` (0 vulns) |
| **Control Plane** | `services/control-plane/` | Go (MSRV 1.23, CI/Prod 1.26.10/1.27.3), Chi 5.3.2, SQLite (WAL), NodesHub, WSS, PTY/tmux | Concluído (v1.2.2) | `go test -count=1 ./...` (PASS - 11/11 pacotes) & `go vet ./...` (PASS) & Cross-compile Linux amd64/arm64 (PASS) |
| **Server Agent** | `services/server-agent/` | Go (MSRV 1.23, CI/Prod 1.26.10/1.27.3), systemd, Ed25519, `/proc`, Openat2/safepath, WSS Client | Concluído (v1.2.2) | `go test -count=1 ./...` (PASS - 6/6 pacotes) & `go vet ./...` (PASS) & Cross-compile Linux amd64/arm64 (PASS) |
| **API Contracts** | `packages/api-contracts/` | OpenAPI 3.0 YAML | Concluído (v1.2.2) | Contratos atualizados com headers de segurança, pareamento com assinatura obrigatória e rotas restritas de logs |
| **Deploy & Infra** | `deploy/` | systemd, Caddy, Bash | Concluído (v1.2.2) | `bash -n` e `shellcheck -x` aprovados em todos os 13 scripts (`install.sh`, `setup-host.sh`, `install-agent.sh`, etc.) com visudo estrito (Opção A) e `--dry-run` real |
| **CI / CD** | `.github/workflows/` | GitHub Actions | Concluído (v1.2.2) | Workflow `ci.yml` cobrindo Go 1.26.10 e 1.27.3, Node 20, `-race`, govulncheck, cross-compilação e ShellCheck |
| **Ferramentas de Segurança** | Raiz / Módulos | staticcheck, gosec, gitleaks, shellcheck, govulncheck, npm audit | Concluído (v1.2.2) | 0 leaks em 11 commits (Gitleaks), 0 issues em Gosec (ambos os módulos Go Windows/Linux), 0 erros em ShellCheck -x, 0 staticcheck issues |
| **Documentação** | `docs/` | Markdown | Concluído | `architecture.md`, `security.md`, `installation.md`, `upgrade.md`, `troubleshooting.md` |
| **Migrações** | `migrations/` | SQL | Concluído | `000001_init_schema.sql`, `000002_game_servers.sql`, `000003_multi_server.sql` |
| **Testes de Aceitação** | `internal/nodes/` | Go Integration | Concluído (10/10) | `TestMultiServer_AcceptanceCriteria10` aprovado |
| **Testes de Segurança** | `internal/api/` | Go Integration | Concluído | `TestSecurity_ServiceLogsAndActions_StrictAuthorizationAndSanitization` e `TestSecurity_Ed25519Pairing_SignatureMandatoryAndImmutability` aprovados |

---

## 3. Módulos Implementados e Integrados

### Runtime Integrado do Server Agent (v1.2.1)
1. **Conexões Reversas WSS com Cabeçalhos Limpos:**
   - O executável do agente conecta-se ao Control Plane (`wss://painel/api/v1/agent/connect`) passando autenticação via cabeçalhos HTTP (`X-LSM-Node-ID`, `X-LSM-Agent-Token`, `Authorization: Bearer`), mantendo segredos fora de URLs e logs.
   - Reconexão resiliente com backoff exponencial automático de 2s até 30s.
2. **Ciclo de Vida Criptográfico Ed25519 Unificado:**
   - Geração e persistência de identidade em `/var/lib/lsm-agent/identity.key` (permissão `0600`).
   - Assinatura criptográfica do token de pareamento pelo agente e validação estrita no painel antes de conceder acesso.
   - Flags CLI no agente (`--export-pubkey`, `--sign-token`, `--config`) permitindo automação limpa sem ferramentas externas.
3. **Operações Reais de Sistema:**
   - Varredura de `/proc` para listar processos reais do host com CPU/Memória/UID e controle de sinais (`SIGTERM`, `SIGKILL`, `SIGHUP`, `SIGINT`) com proteção ao PID 1.
   - Gestão de serviços systemd (`list-units`, `start`, `stop`, `restart`, `reload`, `status`) e leitura de logs via `journalctl`.
   - Execução com menor privilégio utilizando helper dedicado (`/usr/local/libexec/lsm-privileged-helper`) sob regra sudoers restrita (`/etc/sudoers.d/lsm-agent`).
4. **Manipulação Segura de Arquivos e Anti-TOCTOU:**
   - Uso de `os.OpenFile` com flags estritas (`O_RDWR`, `O_CREATE`, `O_TRUNC`, `O_NOFOLLOW` quando aplicável).
   - Verificação de metadados em descritores abertos e bloqueio de symlink traversal via `filepath.EvalSymlinks`.
   - Isolamento de usuários não-root ao diretório `/home/<usuario>`.

---

## 4. Evidências Reais de Execução de Testes

### Backend (`services/control-plane`)
```text
ok  	github.com/linux-server-manager/backend/internal/api	2.134s
ok  	github.com/linux-server-manager/backend/internal/auth	0.384s
ok  	github.com/linux-server-manager/backend/internal/database	0.776s
ok  	github.com/linux-server-manager/backend/internal/files	0.664s
ok  	github.com/linux-server-manager/backend/internal/monitoring	0.730s
ok  	github.com/linux-server-manager/backend/internal/nodes	0.837s
ok  	github.com/linux-server-manager/backend/internal/steamcmd	1.036s
ok  	github.com/linux-server-manager/backend/internal/systemmgr	0.384s
ok  	github.com/linux-server-manager/backend/internal/terminal	0.955s
```

### Server Agent (`services/server-agent`)
```text
ok  	github.com/linux-server-manager/agent/internal/identity	0.390s
ok  	github.com/linux-server-manager/agent/internal/metrics	0.192s
ok  	github.com/linux-server-manager/agent/internal/security	0.193s
ok  	github.com/linux-server-manager/agent/internal/transport	0.761s
```

### Scripts de Deploy (`deploy/scripts/`)
```text
OK: check-installation.sh, install-agent.sh, install-control-plane.sh, install.sh,
    lsm-privileged-helper.sh, setup-host.sh, uninstall.sh, upgrade.sh,
    lib/common.sh, lib/dependencies.sh, lib/logging.sh, lib/os-detection.sh, lib/validation.sh
```

### Frontend (`apps/web`)
```text
Test Files  1 passed (1)
     Tests  4 passed (4)
built in 5.91s
```
