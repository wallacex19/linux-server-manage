-- Migração 000003: Suporte a Multi-Servidor (Control Plane + Server Agent)

-- Expande tabela nodes com campos de agrupamento, pareamento e revogação
ALTER TABLE nodes ADD COLUMN group_name TEXT DEFAULT 'Geral';
ALTER TABLE nodes ADD COLUMN tags TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN agent_version TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN pairing_token TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN pairing_expires_at TIMESTAMP;
ALTER TABLE nodes ADD COLUMN agent_token TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN revoked_at TIMESTAMP;
ALTER TABLE nodes ADD COLUMN last_error TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN os_info TEXT DEFAULT '';

-- Tabela de histórico de conexões e eventos do agente
CREATE TABLE IF NOT EXISTS node_connection_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL,
    event TEXT NOT NULL, -- 'paired', 'connected', 'disconnected', 'heartbeat_timeout', 'revoked'
    ip_address TEXT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- Tabela de cache do último snapshot de métricas de cada nó
CREATE TABLE IF NOT EXISTS node_metrics_cache (
    node_id TEXT PRIMARY KEY,
    cpu_percent REAL DEFAULT 0,
    memory_percent REAL DEFAULT 0,
    disk_percent REAL DEFAULT 0,
    process_count INTEGER DEFAULT 0,
    uptime_seconds INTEGER DEFAULT 0,
    raw_snapshot JSON,
    collected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_node_logs_node ON node_connection_logs(node_id);
CREATE INDEX IF NOT EXISTS idx_nodes_group ON nodes(group_name);
