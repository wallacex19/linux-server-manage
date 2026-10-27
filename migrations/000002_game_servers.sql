-- Inserir nó local padrão se não existir
INSERT OR IGNORE INTO nodes (id, name, hostname, ip_address, public_key, status)
VALUES ('local-node', 'Servidor Local', 'localhost', '127.0.0.1', 'local', 'online');

-- Adicionar colunas complementares para gerenciamento de servidores de jogos
ALTER TABLE managed_servers ADD COLUMN app_id TEXT DEFAULT '';
ALTER TABLE managed_servers ADD COLUMN start_command TEXT DEFAULT '';
ALTER TABLE managed_servers ADD COLUMN port INTEGER DEFAULT 0;
