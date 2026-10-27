# Arquitetura do Sistema — Linux Server Manager

## 1. Visão Geral
O **Linux Server Manager (LSM)** é uma plataforma modular desenhada para o gerenciamento centralizado de servidores Linux e instâncias de servidores de jogos dedicados.

O sistema divide-se em três pilares fundamentais:
1. **Web App (`apps/web`):** Interface do operador construída em React, TypeScript, Vite, Tailwind CSS e componentes shadcn/ui.
2. **Control Plane (`services/control-plane`):** Backend central em Go com roteador Chi, banco de dados SQLite com WAL mode, gerenciamento de inventário e hub de WebSockets seguros (WSS).
3. **Server Agent (`services/server-agent`):** Agente autônomo instalado em cada nó Linux gerenciado, rodando como serviço nativo do systemd, com coleta de métricas direto do `/proc`, controle de sessões `tmux` e isolamento de processos SteamCMD.

---

## 2. Diagrama de Comunicação

```
┌────────────────────────────────────────────────────────┐
│                   Operador / Navegador                 │
│              apps/web (React 18 + xterm.js)            │
└───────────────────────────▲────────────────────────────┘
                            │ HTTPS / WSS
┌───────────────────────────▼────────────────────────────┐
│                    Reverse Proxy (Caddy)               │
│                  TLS Automático + Headers              │
└───────────────────────────▲────────────────────────────┘
                            │ HTTP :8080
┌───────────────────────────▼────────────────────────────┐
│               services/control-plane (Go)              │
│       Chi Router + SQLite (WAL) + Auditoria PAM        │
└───────────────────────────▲────────────────────────────┘
                            │ WSS Seguro (mTLS / Ed25519)
┌───────────────────────────▼────────────────────────────┐
│               services/server-agent (Go)               │
│          Serviço systemd (Usuário lsm-agent)           │
│   ├── Métricas nativas (/proc/stat, /proc/meminfo)     │
│   ├── Sessões persistentes via tmux                    │
│   └── Gerenciamento de servidores SteamCMD             │
└────────────────────────────────────────────────────────┘
```

---

## 3. Topologia e Isolamento
- **Independência dos Nós:** Caso a conexão com o Control Plane seja interrompida, o Server Agent mantém os processos e game servers em execução ininterrupta.
- **Sessões Persistentes:** O controle de consoles interativos é realizado através do `tmux` atachado a canais PTY virtuais, redirecionados via WebSocket seguro até o terminal `xterm.js` na interface Web.
