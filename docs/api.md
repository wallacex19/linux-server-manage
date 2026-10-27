# Referência da API — Linux Server Manager

A documentação canônica dos contratos REST está disponível na especificação OpenAPI em:
- [packages/api-contracts/openapi.yaml](../packages/api-contracts/openapi.yaml)
- [docs/openapi.yaml](openapi.yaml)

---

## 1. Endpoints Base (Control Plane)

### `GET /api/v1/health`
Retorna a integridade operacional do backend central e da conexão SQLite.

**Resposta de Sucesso (`200 OK`):**
```json
{
  "status": "healthy",
  "timestamp": "2026-10-25T07:47:47.382Z",
  "environment": "development",
  "database": "connected",
  "version": "0.1.0"
}
```

---

### `GET /api/v1/version`
Retorna o nome e versão do serviço do Control Plane.

**Resposta de Sucesso (`200 OK`):**
```json
{
  "name": "Linux Server Manager - Backend Central",
  "version": "0.1.0"
}
```

---

### `GET /api/v1/nodes`
Lista os nós de servidores Linux gerenciados registrados no Control Plane.

---

## 2. Comunicação de Nós em Tempo Real (WebSocket)
- **Endpoint:** `/api/v1/agent/connect`
- **Autenticação:** Handshake com assinatura Ed25519 e verificação de chave pública cadastrada.
- **Heartbeat:** Intervalo configurável de 5 segundos contendo métricas nativas do host (`/proc/stat`, `/proc/meminfo`, carga e uptime).
