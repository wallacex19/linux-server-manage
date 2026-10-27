# Guia de Resolução de Problemas (Troubleshooting) — LSM

Este guia auxilia na identificação e correção de problemas operacionais comuns no Linux Server Manager.

## 1. Ferramenta de Diagnóstico Automatizado

Execute o script de diagnóstico para inspecionar permissões, serviços e portas:
```bash
sudo bash deploy/scripts/install.sh --mode check
```

---

## 2. Problemas Comuns e Soluções

### 2.1 Agente Remoto Aparece como "Offline" no Painel
- **Causa:** O serviço do agente não está em execução ou não consegue alcançar o painel via WebSocket (`wss://`).
- **Verificação no nó do agente:**
  ```bash
  systemctl status lsm-agent.service
  journalctl -u lsm-agent.service -n 50 --no-pager
  ```
- **Solução:**
  1. Verifique se o painel está acessível a partir do nó:
     ```bash
     curl -I https://seu-painel.exemplo.com/api/v1/health
     ```
  2. Confirme que `/var/lib/lsm-agent/agent_config.json` possui o `node_id` e `agent_token` corretos.
  3. Se o nó foi revogado no painel, será necessário gerar um novo token e pareá-lo novamente.

### 2.2 Erro 403 Forbidden em Requisições WebSocket (CSWSH)
- **Causa:** O cabeçalho `Origin` enviado pelo navegador não corresponde ao `Host` ou à lista de origens locais autorizadas (`localhost`, `127.0.0.1`).
- **Solução:**
  - Em produção, acesse o painel através do domínio oficial configurado no Caddy ou configure o cabeçalho `Host` correto no proxy reverso (`proxy_set_header Host $host;` ou Caddyfile padrão).

### 2.3 Erro "A conta '<usuario>' não existe no servidor remoto"
- **Causa:** O LSM adota o princípio de segurança fundamental de que não existem bancos paralelos de senhas ou contas virtuais. O operador autenticado no Control Plane precisa ter uma conta local correspondente no servidor Linux remoto para realizar operações nele.
- **Solução:**
  - Crie a conta do operador no servidor Linux remoto:
    ```bash
    sudo adduser <usuario>
    # Se o operador precisar gerenciar serviços do sistema:
    sudo usermod -aG sudo <usuario>
    ```

### 2.4 Erro "Caminho fora do diretório home do usuário" ou "Symlink Traversal"
- **Causa:** Usuários não-administradores estão estritamente confinados ao seu diretório home (`/home/<usuario>`). Links simbólicos que apontam para diretórios externos (`/etc`, `/var`, `/root`) são bloqueados pelo motor de validação.
- **Solução:**
  - Mantenha os arquivos dentro da pasta home do usuário ou adicione o usuário ao grupo `sudo` caso ele necessite de acesso administrativo a todo o sistema de arquivos.

### 2.5 Falha ao Instalar Servidores via SteamCMD
- **Causa:** Bibliotecas de 32 bits ausentes no Ubuntu Server 64 bits.
- **Solução:**
  ```bash
  sudo dpkg --add-architecture i386
  sudo apt-get update
  sudo apt-get install -y steamcmd lib32gcc-s1
  ```

---

## 3. Logs do Sistema para Depuração

- **Logs do Control Plane:**
  ```bash
  journalctl -u lsm-control-plane -f
  ```
- **Logs do Agente:**
  ```bash
  journalctl -u lsm-agent -f
  ```
- **Logs do Proxy Reverso Caddy:**
  ```bash
  journalctl -u caddy -f
  ```
