package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	ErrNodeOffline      = errors.New("o servidor remoto está offline no momento")
	ErrOperationTimeout = errors.New("a operação no servidor remoto excedeu o tempo limite")
)

// checkAgentOrigin previne CSWSH ao validar se conexões originadas por navegadores pertencem ao mesmo host
func checkAgentOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Agentes Go em servidores Linux não enviam cabeçalho Origin de navegador
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	if strings.EqualFold(u.Host, r.Host) {
		return true
	}

	reqHost, _, errReq := net.SplitHostPort(r.Host)
	if errReq != nil {
		reqHost = r.Host
	}
	origHost, _, errOrig := net.SplitHostPort(u.Host)
	if errOrig != nil {
		origHost = u.Host
	}

	isLocalReq := reqHost == "localhost" || reqHost == "127.0.0.1" || reqHost == "::1"
	isLocalOrig := origHost == "localhost" || origHost == "127.0.0.1" || origHost == "::1"

	return isLocalReq && isLocalOrig
}

var upgrader = websocket.Upgrader{
	CheckOrigin:     checkAgentOrigin,
	ReadBufferSize:  1024 * 64,
	WriteBufferSize: 1024 * 64,
}

type AgentMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"` // "heartbeat", "command_req", "command_resp", "event"
	Action    string          `json:"action,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type AgentConnection struct {
	nodeID    string
	conn      *websocket.Conn
	hub       *AgentHub
	send      chan []byte
	pending   map[string]chan AgentMessage
	pendingMu sync.Mutex
	mu        sync.Mutex
}

type AgentHub struct {
	service *Service
	agents  map[string]*AgentConnection
	mu      sync.RWMutex
}

func NewAgentHub(service *Service) *AgentHub {
	hub := &AgentHub{
		service: service,
		agents:  make(map[string]*AgentConnection),
	}
	return hub
}

// HandleAgentWebSocket trata a conexão persistente de um agente via WebSocket
func (h *AgentHub) HandleAgentWebSocket(w http.ResponseWriter, r *http.Request) {
	// Autenticação estritamente via cabeçalhos HTTP para evitar segredos em URLs e logs de proxy
	nodeID := r.Header.Get("X-LSM-Node-ID")
	token := r.Header.Get("X-LSM-Agent-Token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if nodeID == "" || token == "" {
		http.Error(w, `{"error":"identificação e token do agente são obrigatórios via cabeçalhos HTTP"}`, http.StatusUnauthorized)
		return
	}

	if !h.service.ValidateAgent(nodeID, token) {
		http.Error(w, `{"error":"credenciais do agente inválidas ou revogadas"}`, http.StatusForbidden)
		return
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[AgentHub] Falha no upgrade WebSocket para nó %s: %v", nodeID, err)
		return
	}

	agentConn := &AgentConnection{
		nodeID:  nodeID,
		conn:    wsConn,
		hub:     h,
		send:    make(chan []byte, 100),
		pending: make(map[string]chan AgentMessage),
	}

	h.registerAgent(agentConn)

	go agentConn.writePump()
	agentConn.readPump(r.RemoteAddr)
}

// DisconnectNode encerra a conexão WebSocket ativa de um nó (ex: ao ser revogado)
func (h *AgentHub) DisconnectNode(nodeID, reason string) {
	h.mu.Lock()
	ac, exists := h.agents[nodeID]
	if exists {
		delete(h.agents, nodeID)
	}
	h.mu.Unlock()

	if exists && ac != nil {
		_ = ac.conn.Close()
		h.service.RecordDisconnection(nodeID, reason)
		log.Printf("[AgentHub] Conexão do nó %s encerrada ativamente (%s)", nodeID, reason)
	}
}

func (h *AgentHub) registerAgent(ac *AgentConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Se já havia uma conexão antiga para este nó, encerra a anterior
	if old, exists := h.agents[ac.nodeID]; exists {
		_ = old.conn.Close()
	}

	h.agents[ac.nodeID] = ac
	log.Printf("[AgentHub] Agente registrado e conectado: nó %s", ac.nodeID)
}

func (h *AgentHub) unregisterAgent(ac *AgentConnection, reason string) {
	h.mu.Lock()
	if current, exists := h.agents[ac.nodeID]; exists && current == ac {
		delete(h.agents, ac.nodeID)
	}
	h.mu.Unlock()

	_ = ac.conn.Close()
	h.service.RecordDisconnection(ac.nodeID, reason)
	log.Printf("[AgentHub] Agente desconectado: nó %s (%s)", ac.nodeID, reason)
}

// IsNodeOnline verifica se o nó possui conexão ativa WebSocket
func (h *AgentHub) IsNodeOnline(nodeID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.agents[nodeID]
	return exists
}

// SendCommand envia um comando ao agente remoto e aguarda a resposta tipada
func (h *AgentHub) SendCommand(ctx context.Context, nodeID string, action string, payload interface{}) (json.RawMessage, error) {
	h.mu.RLock()
	ac, exists := h.agents[nodeID]
	h.mu.RUnlock()

	if !exists {
		return nil, ErrNodeOffline
	}

	reqID := uuid.New().String()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar payload: %w", err)
	}

	msg := AgentMessage{
		ID:        reqID,
		Type:      "command_req",
		Action:    action,
		Payload:   payloadBytes,
		Timestamp: time.Now(),
	}

	respChan := make(chan AgentMessage, 1)

	ac.pendingMu.Lock()
	ac.pending[reqID] = respChan
	ac.pendingMu.Unlock()

	defer func() {
		ac.pendingMu.Lock()
		delete(ac.pending, reqID)
		ac.pendingMu.Unlock()
	}()

	msgBytes, _ := json.Marshal(msg)
	select {
	case ac.send <- msgBytes:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, errors.New("fila de saída do agente saturada")
	}

	select {
	case resp := <-respChan:
		if resp.Error != "" {
			return nil, errors.New(resp.Error)
		}
		return resp.Payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, ErrOperationTimeout
	}
}

func (ac *AgentConnection) readPump(remoteAddr string) {
	defer func() {
		ac.hub.unregisterAgent(ac, "conexão encerrada")
	}()

	ac.conn.SetReadLimit(1024 * 1024 * 10) // 10MB para arquivos/saídas
	_ = ac.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	ac.conn.SetPongHandler(func(string) error {
		_ = ac.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := ac.conn.ReadMessage()
		if err != nil {
			break
		}

		_ = ac.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg AgentMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Trata batimento de coração com métricas
		if msg.Type == "heartbeat" {
			var summary NodeMetricsSummary
			if len(msg.Payload) > 0 {
				_ = json.Unmarshal(msg.Payload, &summary)
			}
			_ = ac.hub.service.RecordHeartbeat(ac.nodeID, remoteAddr, &summary)
			continue
		}

		// Trata respostas a comandos solicitados pelo painel
		if msg.Type == "command_resp" {
			ac.pendingMu.Lock()
			if ch, exists := ac.pending[msg.ID]; exists {
				ch <- msg
			}
			ac.pendingMu.Unlock()
		}
	}
}

func (ac *AgentConnection) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer func() {
		ticker.Stop()
		_ = ac.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-ac.send:
			if !ok {
				_ = ac.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			ac.mu.Lock()
			_ = ac.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := ac.conn.WriteMessage(websocket.TextMessage, msg)
			ac.mu.Unlock()
			if err != nil {
				return
			}
		case <-ticker.C:
			ac.mu.Lock()
			_ = ac.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := ac.conn.WriteMessage(websocket.PingMessage, nil)
			ac.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}
