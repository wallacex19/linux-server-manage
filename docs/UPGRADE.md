# Guia de Atualização (Upgrade) — Linux Server Manager (LSM)

Este documento descreve o procedimento oficial para atualização segura e idempotente dos componentes do Linux Server Manager: **Control Plane** (Painel Central) e **Server Agent** (Nós remotos).

## 1. Princípios de Atualização
- **Idempotência:** O processo de atualização pode ser executado múltiplas vezes sem corromper configurações ou duplicar dados.
- **Preservação de Dados:** O banco de dados SQLite (`/var/lib/lsm-control-plane/lsm_panel.db`), a chave criptográfica Ed25519 (`/var/lib/lsm-agent/identity.key`) e o arquivo de configuração de pareamento (`/var/lib/lsm-agent/agent_config.json`) são estritamente preservados.
- **Migrações Automáticas:** Ao iniciar a nova versão do Control Plane, novas migrações SQL são aplicadas automaticamente em transação SQLite WAL.

---

## 2. Atualização via Script Automatizado

O repositório inclui o script oficial `deploy/scripts/upgrade.sh`, integrado ao `install.sh`:

### Atualizar o Control Plane (Painel Central):
```bash
sudo bash deploy/scripts/install.sh --role control-plane --mode upgrade
```
Ou chamando diretamente:
```bash
sudo bash deploy/scripts/upgrade.sh --role control-plane
```

O script executa as seguintes etapas:
1. Valida sistema operacional e privilégios de superusuário.
2. Copia o binário atualizado para `/usr/local/bin/lsm-control-plane`.
3. Atualiza os arquivos estáticos compilados do frontend em `/var/www/lsm-web/dist`.
4. Reinicia o serviço `lsm-control-plane.service`.
5. Valida a saúde da API local via endpoint `/api/v1/health`.

### Atualizar o Server Agent (Nós Remotos):
```bash
sudo bash deploy/scripts/install.sh --role agent --mode upgrade
```
Ou chamando diretamente:
```bash
sudo bash deploy/scripts/upgrade.sh --role agent
```

O script:
1. Copia o novo binário para `/usr/local/bin/lsm-agent`.
2. Preserva a chave `/var/lib/lsm-agent/identity.key` e o token de autenticação.
3. Reinicia o serviço `lsm-agent.service`.
4. Valida se o serviço está ativo (`active/running`).

---

## 3. Modo de Simulação (Dry-Run)

Para verificar o que será modificado sem realizar qualquer alteração física no sistema:
```bash
sudo bash deploy/scripts/install.sh --role control-plane --mode upgrade --dry-run
```

---

## 4. Diagnóstico Pós-Upgrade

Após qualquer atualização, execute o diagnóstico automatizado para confirmar a integridade de permissões e serviços:
```bash
sudo bash deploy/scripts/install.sh --mode check
```
