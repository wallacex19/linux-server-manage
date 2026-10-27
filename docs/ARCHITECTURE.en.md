# System Architecture — Linux Server Manager

## 1. Overview
The **Linux Server Manager (LSM)** is a modular platform designed for centralized management of Linux servers and dedicated game server instances.

The system is organized around three primary pillars:
1. **Web App (`apps/web`):** Operator interface built with React 18, TypeScript, Vite, Tailwind CSS, Recharts, and shadcn/ui components.
2. **Control Plane (`services/control-plane`):** Central backend written in Go using the Chi router, SQLite in WAL mode for inventory/audit, and a secure WebSocket (WSS) reverse agent hub.
3. **Server Agent (`services/server-agent`):** Lightweight daemon installed on each managed Linux node, running as a hardened systemd unit, collecting native `/proc` metrics, managing `tmux` sessions, and coordinating game server instances.

---

## 2. Communication Topology

```text
┌────────────────────────────────────────────────────────┐
│                   Operator / Browser                   │
│              apps/web (React 18 + xterm.js)            │
└───────────────────────────▲────────────────────────────┘
                            │ HTTPS / WSS
┌───────────────────────────▼────────────────────────────┐
│                    Reverse Proxy (Caddy)               │
│                  Automatic TLS + Headers               │
└───────────────────────────▲────────────────────────────┘
                            │ HTTP :8080
┌───────────────────────────▼────────────────────────────┐
│               services/control-plane (Go)              │
│       Chi Router + SQLite (WAL) + PAM Audit + Hub      │
└───────────────────────────▲────────────────────────────┘
                            │ Reverse WSS Tunnel (Ed25519)
┌───────────────────────────▼────────────────────────────┐
│               services/server-agent (Go)               │
│          systemd Service (User: lsm-agent)             │
│   ├── Native metrics (/proc/stat, /proc/meminfo)       │
│   ├── Persistent sessions via tmux                     │
│   └── SteamCMD dedicated server management             │
└────────────────────────────────────────────────────────┘
```

---

## 3. Distributed Multi-Server Design

- **Reverse WebSocket Tunnels (Outbound-only):** Managed agents initiate connections outward to the Control Plane (`wss://control-plane/api/v1/agent/connect`). Remote hosts do not need public IP addresses, inbound firewall rules, or port forwarding.
- **Node Independence:** If the connection between an agent and the Control Plane drops, all running background workloads, game servers, and services remain active and uninterrupted.
- **Session Persistence:** Terminals use virtual PTYs backed by user-isolated `tmux` sockets (`tmux -L lsm-<username>`), streamed through WebSockets into browser-rendered `xterm.js`.
- **Local Identity Verification:** Operations dispatched to remote nodes check the operator's account on the destination host via `AccountChecker` (`/etc/passwd`). If the account does not exist locally on that node, access is rejected.
