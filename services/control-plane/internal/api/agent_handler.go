package api

import (
	"encoding/json"
	"net/http"

	"github.com/linux-server-manager/backend/internal/nodes"
)

type AgentHandler struct {
	svc *nodes.Service
	hub *nodes.AgentHub
}

func NewAgentHandler(svc *nodes.Service, hub *nodes.AgentHub) *AgentHandler {
	return &AgentHandler{
		svc: svc,
		hub: hub,
	}
}

// Pair trata o pareamento do agente Linux consumido pelo script de instalação
func (h *AgentHandler) Pair(w http.ResponseWriter, r *http.Request) {
	var req nodes.PairAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	if req.IPAddress == "" {
		req.IPAddress = r.RemoteAddr
	}

	resp, err := h.svc.PairAgent(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == nodes.ErrInvalidOrExpiredToken {
			w.WriteHeader(http.StatusUnauthorized)
		} else if err == nodes.ErrNodeRevoked {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Connect trata a conexão reversa persistente do agente via WebSocket
func (h *AgentHandler) Connect(w http.ResponseWriter, r *http.Request) {
	h.hub.HandleAgentWebSocket(w, r)
}
