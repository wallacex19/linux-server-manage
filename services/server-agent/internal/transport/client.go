package transport

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/linux-server-manager/agent/internal/config"
	"github.com/linux-server-manager/agent/internal/identity"
	"github.com/linux-server-manager/agent/internal/metrics"
	"github.com/linux-server-manager/agent/internal/safepath"
	"github.com/linux-server-manager/agent/internal/security"
	"github.com/linux-server-manager/agent/internal/system"
)

type AgentMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"` // "heartbeat", "command_req", "command_resp"
	Action    string          `json:"action,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type CommandPayload struct {
	OperatorUsername string                    `json:"operator_username"`
	OperatorUID      int                       `json:"operator_uid,omitempty"`
	IsAdmin          bool                      `json:"is_admin,omitempty"`
	Path             string                    `json:"path,omitempty"`
	Content          string                    `json:"content,omitempty"`
	OldPath          string                    `json:"old_path,omitempty"`
	NewPath          string                    `json:"new_path,omitempty"`
	PID              int                       `json:"pid,omitempty"`
	Signal           string                    `json:"signal,omitempty"`
	ServiceName      string                    `json:"service_name,omitempty"`
	Action           string                    `json:"action,omitempty"`
	Lines            string                    `json:"lines,omitempty"`
	Ticket           *system.ProcessKillTicket `json:"ticket,omitempty"`
}

type Client struct {
	cfg       config.Config
	ident     *identity.NodeIdentity
	checker   *security.AccountChecker
	collector metrics.Collector
	conn      *websocket.Conn
	sendChan  chan []byte
	mu        sync.Mutex
	stopChan  chan struct{}
}

func NewClient(cfg config.Config, ident *identity.NodeIdentity) *Client {
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 5
	}
	return &Client{
		cfg:       cfg,
		ident:     ident,
		checker:   security.NewAccountChecker(cfg.Hostname),
		collector: metrics.NewCollector(),
		sendChan:  make(chan []byte, 100),
		stopChan:  make(chan struct{}),
	}
}

// Start estabelece a conexão persistente com o Control Plane com reconexão automática e backoff exponencial
func (c *Client) Start(ctx context.Context) {
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		default:
		}

		err := c.connectAndServe(ctx)
		if err != nil {
			log.Printf("[AgentTransport] Conexão com Control Plane interrompida (%v). Reconectando em %v...", err, backoff)
		}

		select {
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		}
	}
}

func (c *Client) Stop() {
	c.mu.Lock()
	select {
	case <-c.stopChan:
	default:
		close(c.stopChan)
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.mu.Unlock()
}

func (c *Client) connectAndServe(ctx context.Context) error {
	panelURL := c.cfg.PanelURL
	wsScheme := "wss"
	if strings.HasPrefix(panelURL, "http://") {
		if c.cfg.Environment == "production" {
			return errors.New("conexão insegura HTTP/WS rejeitada em ambiente de produção (LSM_ENV=production)")
		}
		wsScheme = "ws"
	} else if !strings.HasPrefix(panelURL, "https://") {
		return fmt.Errorf("esquema de URL do painel inválido (esperado HTTPS): %s", panelURL)
	}
	cleanHost := strings.TrimPrefix(strings.TrimPrefix(panelURL, "https://"), "http://")
	cleanHost = strings.TrimRight(cleanHost, "/")

	u := url.URL{
		Scheme: wsScheme,
		Host:   cleanHost,
		Path:   "/api/v1/agent/connect",
	}

	// Autenticação estritamente via cabeçalhos HTTP sem expor segredos em query strings e logs
	headers := make(http.Header)
	headers.Set("X-LSM-Node-ID", c.cfg.NodeID)
	headers.Set("X-LSM-Agent-Token", c.cfg.AgentToken)
	headers.Set("Authorization", "Bearer "+c.cfg.AgentToken)

	dialer := &websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}

	log.Printf("[AgentTransport] Conectando ao Control Plane em %s://%s/api/v1/agent/connect (Nó: %s)...", wsScheme, cleanHost, c.cfg.NodeID)
	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			return errors.New("acesso rejeitado: nó revogado ou credenciais inválidas no Control Plane")
		}
		return fmt.Errorf("falha ao conectar websocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	log.Printf("[AgentTransport] Conexão com Control Plane estabelecida com sucesso.")

	// Inicia rotina de heartbeat
	go c.heartbeatLoop(ctx)

	// Loop de leitura de mensagens do Control Plane
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("leitura do websocket falhou: %w", err)
		}

		var msg AgentMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			continue
		}

		if msg.Type == "command_req" {
			go c.handleCommand(msg)
		}
	}
}

func (c *Client) handleCommand(req AgentMessage) {
	resp := AgentMessage{
		ID:        req.ID,
		Type:      "command_resp",
		Action:    req.Action,
		Timestamp: time.Now(),
	}

	var cmdPayload CommandPayload
	if err := json.Unmarshal(req.Payload, &cmdPayload); err != nil {
		resp.Error = "payload do comando corrompido ou inválido"
		c.sendMessage(resp)
		return
	}

	// Regra Fundamental de Segurança: validação da conta local do operador no servidor Linux
	localUser, err := c.checker.CheckLocalAccount(cmdPayload.OperatorUsername)
	if err != nil {
		resp.Error = err.Error()
		c.sendMessage(resp)
		return
	}

	// O status administrativo exige consenso estrito entre o Control Plane e a base local
	effectiveAdmin := localUser.IsAdmin && cmdPayload.IsAdmin
	localUser.IsAdmin = effectiveAdmin

	// Execução da ação autorizada sob a conta local
	resultPayload, err := c.executeAction(req.Action, cmdPayload, localUser)
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Payload = resultPayload
	}

	c.sendMessage(resp)
}


func isCriticalSystemPath(p string) bool {
	clean := filepath.ToSlash(filepath.Clean(p))
	clean = strings.TrimPrefix(clean, "")
	critical := []string{"/", "/etc", "/boot", "/dev", "/proc", "/sys", "/root", "/var", "/bin", "/sbin", "/usr", "c:/", "c:/windows", "c:/windows/system32"}
	for _, c := range critical {
		if strings.EqualFold(clean, c) || strings.EqualFold(clean, "/"+strings.TrimPrefix(c, "/")) {
			return true
		}
	}
	return false
}

func (c *Client) executeAction(action string, payload CommandPayload, user *security.LocalAccountInfo) (json.RawMessage, error) {
	baseRoot := user.HomeDir
	if user.IsAdmin || baseRoot == "" {
		baseRoot = "/"
	}

	switch action {
	case "ping":
		return json.Marshal(map[string]interface{}{
			"status":  "pong",
			"node_id": c.cfg.NodeID,
			"user":    user.Username,
			"uid":     user.UID,
		})
	case "get_system_info":
		snap, err := c.collector.Collect()
		if err != nil {
			return nil, err
		}
		return json.Marshal(snap)

	case "files.list":
		items, currentPath, err := safepath.ListBeneath(baseRoot, payload.Path)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]interface{}{
			"current_path": currentPath,
			"entries":      items,
			"total":        len(items),
		})

	case "files.read":
		data, resolvedPath, err := safepath.ReadBeneath(baseRoot, payload.Path, 5*1024*1024)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]interface{}{
			"path":    resolvedPath,
			"content": string(data),
			"size":    len(data),
		})

	case "files.save":
		resolvedPath, err := safepath.WriteBeneath(baseRoot, payload.Path, []byte(payload.Content), 0644)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"message": "arquivo salvo com sucesso", "path": resolvedPath})

	case "files.mkdir":
		resolvedPath, err := safepath.MkdirBeneath(baseRoot, payload.Path, 0755)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"message": "diretório criado", "path": resolvedPath})

	case "files.create":
		resolvedPath, err := safepath.CreateBeneath(baseRoot, payload.Path, 0644)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"message": "arquivo criado", "path": resolvedPath})

	case "files.rename":
		err := safepath.RenameBeneath(baseRoot, payload.OldPath, payload.NewPath)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"message": "renomeado com sucesso"})

	case "files.delete":
		if isCriticalSystemPath(payload.Path) {
			return nil, errors.New("operação bloqueada em diretório crítico do sistema")
		}
		err := safepath.RemoveBeneath(baseRoot, payload.Path)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"message": "excluído com sucesso"})

	case "system.processes":
		procs, err := system.ListProcesses()
		if err != nil {
			return nil, fmt.Errorf("falha ao listar processos: %w", err)
		}
		return json.Marshal(procs)

	case "system.process_kill":
		err := system.KillProcess(payload.PID, payload.Signal, user.UID, user.Username, c.cfg.PrivilegedHelperPath, user.IsAdmin, payload.Ticket, c.cfg.AgentToken, c.cfg.NodeID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"message": fmt.Sprintf("sinal %s enviado para PID %d com sucesso", payload.Signal, payload.PID)})

	case "system.services":
		svcs, err := system.ListServices()
		if err != nil {
			return nil, fmt.Errorf("falha ao listar serviços: %w", err)
		}
		return json.Marshal(svcs)

	case "system.service_action":
		err := system.ControlService(payload.ServiceName, payload.Action, c.cfg.PrivilegedHelperPath, user.IsAdmin)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"message": fmt.Sprintf("ação '%s' executada no serviço %s com sucesso", payload.Action, payload.ServiceName)})

	case "system.service_logs":
		if !user.IsAdmin {
			return nil, errors.New("acesso negado: consulta a logs de serviço restrita a administradores")
		}
		cleanName := strings.TrimSpace(payload.ServiceName)
		if !strings.HasSuffix(cleanName, ".service") {
			cleanName += ".service"
		}
		if !system.IsValidServiceName(cleanName) {
			return nil, errors.New("nome de serviço inválido")
		}
		lines := 50
		if payload.Lines != "" {
			if l, err := strconv.Atoi(payload.Lines); err == nil && l > 0 {
				if l > 500 {
					return nil, errors.New("quantidade de linhas excede o limite máximo permitido de 500")
				}
				lines = l
			} else {
				return nil, errors.New("parâmetro lines inválido")
			}
		}
		logs, err := system.GetServiceLogs(cleanName, lines, c.cfg.PrivilegedHelperPath)
		if err != nil {
			return nil, errors.New("falha ao consultar logs do serviço")
		}
		return json.Marshal(map[string]string{"service": cleanName, "logs": logs})

	default:
		return nil, fmt.Errorf("ação '%s' não suportada pelo agente", action)
	}
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	interval := time.Duration(c.cfg.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.sendHeartbeat()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

func (c *Client) sendHeartbeat() {
	snap, err := c.collector.Collect()
	if err != nil {
		return
	}

	heartbeatData := map[string]interface{}{
		"cpu_percent":    snap.CPUUsagePercent,
		"memory_percent": snap.MemUsedPercent,
		"disk_percent":   snap.DiskPercent,
		"process_count":  snap.ProcessCount,
		"uptime_seconds": snap.UptimeSeconds,
		"collected_at":   snap.Timestamp,
	}

	payloadBytes, _ := json.Marshal(heartbeatData)
	msg := AgentMessage{
		ID:        fmt.Sprintf("hb-%d", time.Now().UnixNano()),
		Type:      "heartbeat",
		Payload:   payloadBytes,
		Timestamp: time.Now(),
	}

	c.sendMessage(msg)
}

func (c *Client) sendMessage(msg AgentMessage) {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_ = c.conn.WriteMessage(websocket.TextMessage, bytes)
	}
}
