# Pacote de Contratos de API (`@lsm/api-contracts`)

Este diretório contém as definições canônicas de contratos de API REST, eventos WebSocket e modelos de dados compartilhados entre:
- `apps/web` (Frontend React)
- `services/control-plane` (Backend Central em Go)
- `services/server-agent` (Agente Remoto em Go)

## Arquivos
- `openapi.yaml`: Especificação formal OpenAPI 3.0 para todas as rotas HTTP expostas pelo Control Plane.
