# Diretrizes de Segurança — Linux Server Manager

A segurança é o princípio central do projeto Linux Server Manager, orientando todas as decisões arquiteturais.

---

## 1. Princípio do Menor Privilégio e Identidade Efetiva
- **Sem execução como Root Geral:** Nem o Control Plane nem os Server Agents rodam sob a conta `root`.
- **Identidade do Usuário Autenticado:**
  - O usuário autentica-se com sua conta Linux existente no host.
  - O backend identifica UID, GID, grupos (`groups`) e diretório home (`home_dir`).
  - As operações solicitadas (acesso a arquivos, criação de processos, sessões tmux) operam sob o contexto da conta do usuário, impedindo acesso indevido a dados de terceiros.
- **Diferenciação de Privilégios (Admin vs Operador):**
  - Usuários membros de grupos como `sudo` ou `wheel` são sinalizados com `is_admin = true`.
  - Usuários comuns possuem acesso restrito aos seus próprios recursos e sessões.
  - Não há elevação arbitrária de privilégios ou shell root irrestrito exposto na interface.

---

## 2. Autenticação PAM (Pluggable Authentication Modules)
- **Zero Senhas em Banco:** Não há banco de dados paralelo de senhas ou hash de senhas duplicado.
- **Validação no Host:** O módulo `services/control-plane/internal/auth` valida as credenciais diretamente via PAM no Linux.
- **Proteção contra Força Bruta (Rate Limiter):**
  - Cada IP e usuário possui limite estrito de tentativas consecutivas de login (padrão: 5 falhas).
  - Após 5 falhas consecutivas, a conta/IP é bloqueada temporariamente (lockout de 5 minutos).
  - Tentativas bem-sucedidas redefinem o contador imediatamente.

---

## 3. Gestão de Sessões
- **Tokens Criptográficos:** Sessões utilizam tokens aleatórios de 256 bits gerados via `crypto/rand`.
- **Cookies Seguros:**
  - `HttpOnly`: Impede acesso ao cookie via scripts maliciosos (mitigação contra XSS).
  - `SameSite=Lax`: Proteção contra Cross-Site Request Forgery (CSRF).
  - `Path=/`: Restrito ao domínio da aplicação.
  - `Secure`: Ativado automaticamente em ambientes de produção com terminação TLS (Caddy).
- **Revogação e Expiração:** Sessões expiram após período configurável (24 horas) e são revogadas imediatamente no endpoint `/api/v1/auth/logout`.

---

## 4. Auditoria Imutável de Eventos Sensíveis
- Todos os logins bem-sucedidos (`login_success`), falhas de credenciais (`login_failed`), bloqueios de rate limit (`login_blocked_rate_limit`) e encerramentos de sessão (`logout`) são gravados na tabela `audit_logs` do SQLite.
- **Regra Rígida:** Nenhuma senha, token ou segredo é gravado em logs operacionais ou tabelas de auditoria.

---

## 5. Proteções Avançadas de Rede e Requisições (v1.2.0)

### 5.1 Prevenção contra CSWSH (Cross-Site WebSocket Hijacking)
- Conexões WebSocket para terminais interativos (`/api/v1/terminal/ws`) e túneis de agentes (`/api/v1/agent/connect`) validam rigorosamente o cabeçalho `Origin` contra o `Host` da requisição e origens locais aprovadas (`CheckWebSocketOrigin`).
- Requisições originadas de domínios externos não autorizados são rejeitadas com HTTP 403 no handshake HTTP antes de qualquer upgrade de protocolo.

### 5.2 Política Estrita de CORS e Proteção contra CSRF
- O middleware de CORS não reflete origens arbitrárias de atacantes quando `Access-Control-Allow-Credentials: true` está ativo.
- O `csrfProtectionMiddleware` protege todos os endpoints com métodos mutativos de estado (`POST`, `PUT`, `DELETE`, `PATCH`), rejeitando requisições cuja `Origin` ou `Referer` não pertençam ao domínio do painel.

### 5.3 Prevenção contra Symlink Traversal no Gerenciador de Arquivos
- Antes de qualquer operação de leitura, escrita, criação ou exclusão, o caminho é resolvido fisicamente através de `filepath.EvalSymlinks`.
- Links simbólicos criados na pasta de usuários que apontam para arquivos sensíveis fora do escopo autorizado (`/etc`, `/root`, `/var`) são bloqueados com `ErrPathTraversal`.

### 5.4 Sanitização Estrita do SteamCMD contra Injeção de Comandos
- O identificador `AppID` é validado estritamente por regex numérica (`^[0-9]+$`).
- A branch `Beta` é restrita a caracteres alfanuméricos seguros (`^[a-zA-Z0-9_.-]*$`).
- O caminho de instalação (`InstallPath`) é verificado contra metacaracteres de injeção de shell (`;&|`$><"`).
- Atualizações concorrentes no mesmo servidor de jogo são bloqueadas no banco de dados.

### 5.5 Autorização Negativa e Roteamento de Nós Remotos
- Chamadas para gerenciamento de arquivos e processos/serviços aceitam o parâmetro `node_id`.
- Se o nó remoto não existir, o sistema responde com **HTTP 404 (Not Found)**.
- Se o nó estiver revogado, o sistema responde com **HTTP 403 (Forbidden)**.
- Se o nó estiver temporariamente offline, o sistema responde com **HTTP 503 (Service Unavailable)**.
- O agente remoto executa comandos apenas sob o contexto da conta Linux verificada no host local pelo `AccountChecker`.
