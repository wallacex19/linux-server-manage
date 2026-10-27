# Security Guidelines & Architecture — Linux Server Manager

Security is the core architectural principle of the Linux Server Manager project.

---

## 1. Principle of Least Privilege & Effective Identity
- **Zero Generic Root Execution:** Neither the Control Plane nor the Server Agent run as `root`. Dedicated system accounts (`lsm-server` and `lsm-agent`) are used with `/usr/sbin/nologin` shell.
- **Operator Context Mapping:**
  - Users authenticate using their existing Linux user account on the host.
  - The backend identifies the user's UID, GID, groups, and home directory (`/home/<user>`).
  - Requested operations (file access, process management, tmux sessions) execute under the caller's identity context.
- **Administrative Role Enforcement:**
  - Users belonging to `sudo` or `wheel` groups are granted administrative authority (`is_admin = true`).
  - Standard users are strictly confined to their own user scope and home directory.

---

## 2. PAM Host Authentication
- **Zero Stored Passwords:** There is no duplicate password table, synced hash file, or secondary credential database.
- **Direct PAM Validation:** The `auth` package validates credentials against the host OS via standard PAM.
- **Brute-Force Protection:**
  - Per-IP and per-account rate limiters lock attempts after 5 consecutive failures for a 5-minute cool-down.
  - Successful logins immediately reset the penalty counter.

---

## 3. Session Security & CSRF Defense
- **Cryptographic Tokens:** Sessions use 256-bit entropy generated via `crypto/rand`.
- **Hardened Cookies:**
  - `HttpOnly`: Mitigates client-side XSS cookie harvesting.
  - `SameSite=Lax`: Mitigates Cross-Site Request Forgery (CSRF).
  - `Secure`: Enforced in production under TLS termination (Caddy).
- **CSRF Protection Middleware:** State-modifying requests (`POST`, `PUT`, `DELETE`, `PATCH`) must originate from trusted domains matching the host. Untrusted origins are rejected with **HTTP 403 Forbidden**.

---

## 4. Network & Protocol Hardening (CSWSH, CORS)
- **Cross-Site WebSocket Hijacking (CSWSH) Defense:** Handshake origins are checked using `CheckWebSocketOrigin` against the request Host and local development loopback addresses. Arbitrary origins are rejected with HTTP 403 before protocol upgrade.
- **Strict CORS Policy:** Credentials (`Access-Control-Allow-Credentials: true`) are never reflected to arbitrary or untrusted origins.

---

## 5. Filesystem Sandboxing & Symlink Traversal Protection
- **Physical Path Evaluation:** All file system operations invoke `filepath.EvalSymlinks` to canonicalize both the target path and the user's allowed home directory.
- **Symlink Escape Prevention:** If a symbolic link inside a user's directory resolves to a target outside their allowed root (`/etc/shadow`, `/root`, `/var`), the operation is blocked with `ErrPathTraversal`.

---

## 6. SteamCMD & Command Injection Sanitization
- Dedicated game server parameters are validated:
  - `AppID`: Enforced numeric-only format (`^[0-9]+$`).
  - `Beta`: Enforced alphanumeric format (`^[a-zA-Z0-9_.-]*$`).
  - `InstallPath`: Sanitized to reject shell metacharacters (`;&|`$><"`).
  - Concurrent updates to the same instance are blocked at the database level.

---

## 7. Distributed Node Security & Atomic Pairing
- **Asymmetric Identity:** Each managed node generates its own **Ed25519** key pair stored with strict `600` permissions.
- **Atomic Pairing Tokens:** Single-use pairing tokens (`LSM-PAIR-XXXX`) with a 15-minute TTL are consumed in an atomic database transaction (`RowsAffected == 1`) to prevent replay or race conditions.
- **Local Account Checker:** Remote RPC commands verify that the operator has an account on the destination host before performing actions.
