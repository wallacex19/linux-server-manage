# Estrutura de Testes — Linux Server Manager

Este diretório abriga os testes de ponta a ponta (E2E), suítes de integração entre serviços e testes de segurança automatizados do projeto.

---

## Estrutura
- **Unitários:** Executados diretamente dentro de cada serviço (`services/control-plane`, `services/server-agent`, `apps/web`).
- **Integração:** Validação da comunicação entre o Agente e o Control Plane via WebSockets e SQLite.
- **E2E:** Validação de fluxo completo da interface Web com Playwright contra a API.

## Comandos de Execução
```bash
# Testes do Control Plane
cd services/control-plane && go test -v ./...

# Testes do Server Agent
cd services/server-agent && go test -v ./...

# Testes da Aplicação Web
cd apps/web && npm run test
```
