package nodes

import (
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linux-server-manager/backend/internal/database"
)

var (
	ErrNodeNotFound           = errors.New("servidor não encontrado")
	ErrInvalidOrExpiredToken  = errors.New("token de pareamento inválido ou expirado")
	ErrNodeRevoked            = errors.New("o acesso deste servidor foi revogado pelo administrador")
	ErrDuplicateNodeName      = errors.New("já existe um servidor com este nome cadastrado")
	ErrUnauthorizedAgentToken = errors.New("token de autenticação do agente inválido ou revogado")
)

type Service struct {
	db             *database.DB
	panelPublicURL string
	mu             sync.RWMutex
}

func NewService(db *database.DB, panelPublicURL string) *Service {
	if panelPublicURL == "" {
		panelPublicURL = "http://localhost:8080"
	}
	return &Service{
		db:             db,
		panelPublicURL: strings.TrimRight(panelPublicURL, "/"),
	}
}

// CreateNode registra um novo servidor no Control Plane e emite um token de pareamento com TTL de 15 minutos
func (s *Service) CreateNode(req CreateNodeRequest) (*Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("o nome do servidor é obrigatório")
	}

	if req.GroupName == "" {
		req.GroupName = "Geral"
	}

	nodeID := uuid.New().String()
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("falha ao gerar token de pareamento: %w", err)
	}
	pairingToken := fmt.Sprintf("LSM-PAIR-%s", strings.ToUpper(hex.EncodeToString(tokenBytes)))
	expiresAt := time.Now().Add(15 * time.Minute)

	query := `
		INSERT INTO nodes (
			id, name, hostname, ip_address, public_key, status,
			group_name, tags, pairing_token, pairing_expires_at, created_at, updated_at
		) VALUES (?, ?, '', '', '', 'unpaired', ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	_, err := s.db.Exec(query, nodeID, req.Name, req.GroupName, req.Tags, pairingToken, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("falha ao salvar servidor no banco: %w", err)
	}

	installCmd := fmt.Sprintf(
		"curl -sSL %s/deploy/scripts/setup-host.sh | sudo bash -s -- --panel %s --token %s",
		s.panelPublicURL, s.panelPublicURL, pairingToken,
	)

	s.logEvent(nodeID, "created", "", "Servidor cadastrado aguardando pareamento")

	return &Node{
		ID:               nodeID,
		Name:             req.Name,
		Status:           StatusUnpaired,
		GroupName:        req.GroupName,
		Tags:             req.Tags,
		PairingToken:     pairingToken,
		PairingExpiresAt: &expiresAt,
		InstallCommand:   installCmd,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// ListNodes retorna a lista consolidada de todos os servidores com métricas e status em tempo real
func (s *Service) ListNodes(groupFilter string, statusFilter string) ([]Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT
			n.id, n.name, n.hostname, n.ip_address, n.public_key, n.status,
			n.group_name, n.tags, n.agent_version, n.pairing_token, n.pairing_expires_at,
			n.revoked_at, n.last_heartbeat, n.last_error, n.os_info, n.created_at, n.updated_at,
			c.cpu_percent, c.memory_percent, c.disk_percent, c.process_count, c.uptime_seconds, c.collected_at,
			(SELECT COUNT(*) FROM managed_servers ms WHERE ms.node_id = n.id) as game_servers_count
		FROM nodes n
		LEFT JOIN node_metrics_cache c ON c.node_id = n.id
		WHERE 1=1
	`
	var args []interface{}
	if groupFilter != "" && groupFilter != "all" {
		query += " AND n.group_name = ?"
		args = append(args, groupFilter)
	}
	if statusFilter != "" && statusFilter != "all" {
		query += " AND n.status = ?"
		args = append(args, statusFilter)
	}
	query += " ORDER BY n.name ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar servidores: %w", err)
	}
	defer rows.Close()

	nodesList := make([]Node, 0)
	now := time.Now()

	for rows.Next() {
		var n Node
		var statusStr string
		var pairingToken, agentVer, lastErr, osInfo sql.NullString
		var pairExp, revAt, lastHb sql.NullTime
		var cpu, mem, disk sql.NullFloat64
		var procs sql.NullInt32
		var uptime sql.NullInt64
		var collectedAt sql.NullTime
		var gameCount int

		err := rows.Scan(
			&n.ID, &n.Name, &n.Hostname, &n.IPAddress, &n.PublicKey, &statusStr,
			&n.GroupName, &n.Tags, &agentVer, &pairingToken, &pairExp,
			&revAt, &lastHb, &lastErr, &osInfo, &n.CreatedAt, &n.UpdatedAt,
			&cpu, &mem, &disk, &procs, &uptime, &collectedAt, &gameCount,
		)
		if err != nil {
			return nil, fmt.Errorf("falha ao ler linha de servidor: %w", err)
		}

		n.Status = NodeStatus(statusStr)
		if agentVer.Valid {
			n.AgentVersion = agentVer.String
		}
		if lastErr.Valid {
			n.LastError = lastErr.String
		}
		if osInfo.Valid {
			n.OSInfo = osInfo.String
		}
		if revAt.Valid {
			n.RevokedAt = &revAt.Time
		}
		if lastHb.Valid {
			n.LastHeartbeat = &lastHb.Time
		}
		if pairExp.Valid {
			n.PairingExpiresAt = &pairExp.Time
		}
		if pairingToken.Valid {
			n.PairingToken = pairingToken.String
			n.InstallCommand = fmt.Sprintf(
				"curl -sSL %s/deploy/scripts/setup-host.sh | sudo bash -s -- --panel %s --token %s",
				s.panelPublicURL, s.panelPublicURL, n.PairingToken,
			)
		}

		// Detecção de desconexão: se status é 'online', mas o último heartbeat foi há mais de 30 segundos
		if n.Status == StatusOnline && (n.LastHeartbeat == nil || now.Sub(*n.LastHeartbeat) > 30*time.Second) {
			n.Status = StatusOffline
		}

		// Se expirou o token de pareamento e ainda está 'unpaired'
		if n.Status == StatusUnpaired && n.PairingExpiresAt != nil && now.After(*n.PairingExpiresAt) {
			n.Status = StatusError
			n.LastError = "Token de pareamento expirou antes da conexão"
		}

		// Anexa resumo de métricas
		if cpu.Valid && mem.Valid {
			isStale := n.Status != StatusOnline || (collectedAt.Valid && now.Sub(collectedAt.Time) > 30*time.Second)
			n.Metrics = &NodeMetricsSummary{
				CPUPercent:       cpu.Float64,
				MemoryPercent:    mem.Float64,
				DiskPercent:      disk.Float64,
				ProcessCount:     int(procs.Int32),
				UptimeSeconds:    uptime.Int64,
				GameServersCount: gameCount,
				CollectedAt:      collectedAt.Time,
				IsStale:          isStale,
			}
		}

		nodesList = append(nodesList, n)
	}

	return nodesList, nil
}

// GetNode retorna um servidor por ID
func (s *Service) GetNode(id string) (*Node, error) {
	list, err := s.ListNodes("", "")
	if err != nil {
		return nil, err
	}
	for _, n := range list {
		if n.ID == id {
			return &n, nil
		}
	}
	return nil, ErrNodeNotFound
}

// UpdateNode atualiza nome, grupo ou tags de um servidor
func (s *Service) UpdateNode(id string, req UpdateNodeRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE nodes SET name = ?, group_name = ?, tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	res, err := s.db.Exec(query, req.Name, req.GroupName, req.Tags, id)
	if err != nil {
		return fmt.Errorf("falha ao atualizar servidor: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// RevokeNode revoga imediatamente o acesso do agente e invalida seu token permanente
func (s *Service) RevokeNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		UPDATE nodes
		SET status = 'revoked', revoked_at = CURRENT_TIMESTAMP, agent_token = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	res, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("falha ao revogar servidor: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNodeNotFound
	}

	s.logEvent(id, "revoked", "", "Acesso do agente revogado pelo administrador")
	return nil
}

// DeleteNode remove o servidor do painel sem apagar seus arquivos ou serviços locais no host remoto
func (s *Service) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM nodes WHERE id = ?`
	res, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("falha ao excluir servidor: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrNodeNotFound
	}

	s.logEvent(id, "deleted", "", "Servidor removido do catálogo central")
	return nil
}

// ValidateEd25519PublicKey valida e normaliza o formato da chave pública Ed25519 (esperado 32 bytes em hex ou base64)
func ValidateEd25519PublicKey(pubKeyStr string) (ed25519.PublicKey, string, error) {
	pubKeyStr = strings.TrimSpace(pubKeyStr)
	if pubKeyStr == "" {
		return nil, "", errors.New("chave pública do agente ausente")
	}

	// 1. Chave em hexadecimal puro (64 caracteres = 32 bytes)
	if len(pubKeyStr) == 64 {
		if b, err := hex.DecodeString(pubKeyStr); err == nil && len(b) == ed25519.PublicKeySize {
			return ed25519.PublicKey(b), hex.EncodeToString(b), nil
		}
	}

	// 2. Chave no formato OpenSSH: "ssh-ed25519 <base64> [comment]"
	cleanB64 := pubKeyStr
	if strings.HasPrefix(cleanB64, "ssh-ed25519 ") {
		parts := strings.Fields(cleanB64)
		if len(parts) >= 2 {
			cleanB64 = parts[1]
		}
	}

	// 3. Chave em Base64 padrão
	if b64, err := base64.StdEncoding.DecodeString(cleanB64); err == nil {
		if len(b64) == ed25519.PublicKeySize {
			return ed25519.PublicKey(b64), hex.EncodeToString(b64), nil
		}
		// Formato wire OpenSSH: uint32(11) + "ssh-ed25519" + uint32(32) + key(32)
		if len(b64) >= 51 && string(b64[4:15]) == "ssh-ed25519" {
			keyPart := b64[len(b64)-32:]
			return ed25519.PublicKey(keyPart), hex.EncodeToString(keyPart), nil
		}
	}

	return nil, "", errors.New("formato de chave pública Ed25519 inválido: esperado 32 bytes em formato hex ou base64")
}

// PairAgent executa a vinculação segura do agente Linux usando o pairing token de uso único
func (s *Service) PairAgent(req PairAgentRequest) (*AgentPairResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanToken := strings.TrimSpace(req.PairingToken)
	if cleanToken == "" {
		return nil, ErrInvalidOrExpiredToken
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("falha ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	var nodeID, statusStr string
	var expiresAt time.Time
	var revokedAt sql.NullTime
	var existingPubKey sql.NullString

	query := `SELECT id, status, pairing_expires_at, revoked_at, public_key FROM nodes WHERE pairing_token = ?`
	err = tx.QueryRow(query, cleanToken).Scan(&nodeID, &statusStr, &expiresAt, &revokedAt, &existingPubKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidOrExpiredToken
		}
		return nil, err
	}

	if revokedAt.Valid {
		return nil, ErrNodeRevoked
	}

	if time.Now().After(expiresAt) {
		return nil, ErrInvalidOrExpiredToken
	}

	pubKeyObj, normalizedPubKey, err := ValidateEd25519PublicKey(req.PublicKey)
	if err != nil {
		return nil, err
	}

	// 1. Assinatura Ed25519 estritamente obrigatória (64 bytes)
	cleanSig := strings.TrimSpace(req.Signature)
	if cleanSig == "" {
		return nil, errors.New("assinatura criptográfica Ed25519 é obrigatória para pareamento")
	}
	sigBytes, err := hex.DecodeString(cleanSig)
	if err != nil {
		sigBytes, err = base64.StdEncoding.DecodeString(cleanSig)
	}
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return nil, errors.New("assinatura criptográfica Ed25519 malformada ou com tamanho incorreto (esperado 64 bytes)")
	}

	// 2. Proteção contra vinculação forjada de ID de outro nó
	if req.NodeID != "" && req.NodeID != nodeID {
		return nil, errors.New("identificador do nó diverge do token de pareamento fornecido")
	}

	// 3. Imutabilidade da identidade: um nó já pareado não pode ter sua chave pública substituída silenciosamente
	if existingPubKey.Valid && existingPubKey.String != "" && existingPubKey.String != normalizedPubKey {
		return nil, errors.New("nó já possui identidade criptográfica vinculada; substituição não permitida")
	}

	// 4. Verificação canônica da assinatura Ed25519 vinculada ao contexto LSM, ao NodeID e ao PairingToken
	canonicalMsgWithID := fmt.Sprintf("LSM-PAIR-v1:%s:%s", nodeID, cleanToken)
	canonicalMsgCompat := fmt.Sprintf("LSM-PAIR-v1:%s", cleanToken)

	validSig := ed25519.Verify(pubKeyObj, []byte(canonicalMsgWithID), sigBytes) ||
		ed25519.Verify(pubKeyObj, []byte(canonicalMsgCompat), sigBytes) ||
		ed25519.Verify(pubKeyObj, []byte(cleanToken), sigBytes)

	if !validSig {
		return nil, errors.New("falha na verificação da assinatura criptográfica da identidade do nó")
	}

	// Gera token permanente exclusivo de alta entropia para este agente
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("falha ao gerar token de agente: %w", err)
	}
	agentToken := hex.EncodeToString(tokenBytes)

	// Consumo atômico do token de uso único com validação estrita de concorrência
	updateQuery := `
		UPDATE nodes
		SET public_key = ?, hostname = ?, ip_address = ?, os_info = ?, agent_version = ?,
		    agent_token = ?, status = 'connecting', pairing_token = '', pairing_expires_at = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND pairing_token = ?
	`
	res, err := tx.Exec(updateQuery, normalizedPubKey, req.Hostname, req.IPAddress, req.OSInfo, req.AgentVersion, agentToken, nodeID, cleanToken)
	if err != nil {
		return nil, fmt.Errorf("falha ao atualizar pareamento do nó: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, ErrInvalidOrExpiredToken
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("falha ao efetivar transação de pareamento: %w", err)
	}

	s.logEvent(nodeID, "paired", req.IPAddress, fmt.Sprintf("Agente pareado com sucesso (%s - %s)", req.Hostname, req.OSInfo))

	return &AgentPairResponse{
		NodeID:     nodeID,
		AgentToken: agentToken,
		PanelURL:   s.panelPublicURL,
	}, nil
}

// ValidateAgent verifica a autenticidade do token do agente para conexão WebSocket
func (s *Service) ValidateAgent(nodeID, agentToken string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	query := `SELECT COUNT(*) FROM nodes WHERE id = ? AND agent_token = ? AND revoked_at IS NULL`
	err := s.db.QueryRow(query, nodeID, agentToken).Scan(&count)
	return err == nil && count > 0
}

// GetNodeAgentToken retorna o token do agente para assinatura de tickets criptográficos
func (s *Service) GetNodeAgentToken(nodeID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var token string
	query := `SELECT agent_token FROM nodes WHERE id = ? AND revoked_at IS NULL`
	err := s.db.QueryRow(query, nodeID).Scan(&token)
	if err != nil {
		return "", fmt.Errorf("token de agente não encontrado para nó %s: %w", nodeID, err)
	}
	return token, nil
}

// RecordHeartbeat registra o heartbeat do agente e salva o snapshot no cache de métricas
func (s *Service) RecordHeartbeat(nodeID string, ip string, metrics *NodeMetricsSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	_, err := s.db.Exec(
		`UPDATE nodes SET status = 'online', last_heartbeat = ?, ip_address = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		now, ip, nodeID,
	)
	if err != nil {
		return err
	}

	if metrics != nil {
		cacheQuery := `
			INSERT INTO node_metrics_cache (
				node_id, cpu_percent, memory_percent, disk_percent, process_count, uptime_seconds, collected_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(node_id) DO UPDATE SET
				cpu_percent = excluded.cpu_percent,
				memory_percent = excluded.memory_percent,
				disk_percent = excluded.disk_percent,
				process_count = excluded.process_count,
				uptime_seconds = excluded.uptime_seconds,
				collected_at = excluded.collected_at
		`
		_, _ = s.db.Exec(cacheQuery, nodeID, metrics.CPUPercent, metrics.MemoryPercent, metrics.DiskPercent, metrics.ProcessCount, metrics.UptimeSeconds, now)
	}

	return nil
}

// RecordDisconnection registra evento de queda de conexão do agente
func (s *Service) RecordDisconnection(nodeID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.db.Exec(`UPDATE nodes SET status = 'offline', last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, reason, nodeID)
	s.logEvent(nodeID, "disconnected", "", reason)
}

func (s *Service) logEvent(nodeID, event, ip, details string) {
	_, _ = s.db.Exec(
		`INSERT INTO node_connection_logs (node_id, event, ip_address, error_message) VALUES (?, ?, ?, ?)`,
		nodeID, event, ip, details,
	)
}
