# Production Installation Guide — Linux Server Manager

Instructions for automated and manual deployment of the Linux Server Manager on **Ubuntu Server 24.04 LTS** (`amd64` and `arm64` architectures).

---

## 1. Automated Production Installer

The repository provides a complete, idempotent installation suite in `deploy/scripts/`:

### 1.1 Install the Control Plane (Web Dashboard + REST API + Reverse Proxy)
Run with `sudo` or as root:
```bash
sudo bash deploy/scripts/install.sh --role control-plane --domain lsm.example.com
```

The installer automatically handles:
- OS and architecture validation (Ubuntu Server 24.04 LTS).
- System package installation (`curl`, `tmux`, `sqlite3`, `caddy`).
- System user `lsm-server` creation with `/usr/sbin/nologin`.
- Protected directory setup `/var/lib/lsm-control-plane` (mode `750`).
- Binary deployment to `/usr/local/bin/lsm-control-plane` and frontend to `/var/www/lsm-web/dist`.
- Systemd service configuration for `lsm-control-plane.service`.
- Caddy reverse proxy setup with automated Let's Encrypt TLS.
- Local API health check verification.

### 1.2 Install Server Agent on Remote Nodes
On the target Linux machine you want to manage:
```bash
sudo bash deploy/scripts/install.sh --role agent --panel-url https://lsm.example.com --token LSM-PAIR-XXXX
```

The agent installer performs:
- Creation of dedicated `lsm-agent` user and `/var/lib/lsm-agent` (mode `750`).
- Cryptographic **Ed25519** key generation in `/var/lib/lsm-agent/identity.key` (mode `600`).
- Automated initial pairing using the single-use token.
- Deployment and startup of `lsm-agent.service`.
- Establishing the outbound reverse WebSocket tunnel (`wss://`).

---

## 2. Post-Install Diagnostics

To verify service status, port listeners, and permission health at any time:
```bash
sudo bash deploy/scripts/install.sh --mode check
```

---

## 3. Dry-Run Simulation

Simulate all steps without touching files or services:
```bash
sudo bash deploy/scripts/install.sh --role control-plane --dry-run
```

---

## 4. Upgrades and Maintenance

- **Idempotent Upgrade (preserves SQLite databases, keys, and configurations):**
  ```bash
  sudo bash deploy/scripts/install.sh --role control-plane --mode upgrade
  ```
- **Safe Uninstallation (preserves user data directories by default):**
  ```bash
  sudo bash deploy/scripts/install.sh --role control-plane --mode uninstall
  ```
  *(Pass `--purge` only if you wish to remove all databases and state files).*
