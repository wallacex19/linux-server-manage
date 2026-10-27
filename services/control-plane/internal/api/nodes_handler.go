package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/nodes"
)

type NodesHandler struct {
	svc *nodes.Service
	hub *nodes.AgentHub
	db  *database.DB
}

func NewNodesHandler(svc *nodes.Service, hub *nodes.AgentHub, db *database.DB) *NodesHandler {
	return &NodesHandler{
		svc: svc,
		hub: hub,
		db:  db,
	}
}

func (h *NodesHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	groupFilter := r.URL.Query().Get("group")
	statusFilter := r.URL.Query().Get("status")

	nodesList, err := h.svc.ListNodes(groupFilter, statusFilter)
	if err != nil {
		http.Error(w, `{"error":"falha ao listar servidores"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodesList)
}

func (h *NodesHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req nodes.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	node, err := h.svc.CreateNode(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "create_node", "node", node.ID, r.RemoteAddr, "Servidor cadastrado: "+node.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(node)
}

func (h *NodesHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	node, err := h.svc.GetNode(nodeID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "servidor não encontrado"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(node)
}

func (h *NodesHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	nodeID := chi.URLParam(r, "id")

	var req nodes.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	if err := h.svc.UpdateNode(nodeID, req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "update_node", "node", nodeID, r.RemoteAddr, "Servidor renomeado/atualizado")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "servidor atualizado com sucesso"})
}

func (h *NodesHandler) RevokeNode(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	nodeID := chi.URLParam(r, "id")

	if err := h.svc.RevokeNode(nodeID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Encerra imediatamente a conexão ativa do nó se estiver conectado
	h.hub.DisconnectNode(nodeID, "Revogado pelo administrador")

	h.logAudit(user.Username, "revoke_node", "node", nodeID, r.RemoteAddr, "Credencial do agente revogada")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "acesso do servidor revogado com sucesso"})
}

func (h *NodesHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	nodeID := chi.URLParam(r, "id")

	if err := h.svc.DeleteNode(nodeID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Encerra imediatamente a conexão ativa do nó excluído
	h.hub.DisconnectNode(nodeID, "Servidor excluído")

	h.logAudit(user.Username, "delete_node", "node", nodeID, r.RemoteAddr, "Servidor removido do catálogo central")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "servidor removido com sucesso"})
}

func (h *NodesHandler) logAudit(actor, action, resType, resID, ip, details string) {
	if h.db == nil {
		return
	}
	query := `INSERT INTO audit_logs (actor, action, resource_type, resource_id, ip_address, details) VALUES (?, ?, ?, ?, ?, ?)`
	_, _ = h.db.Exec(query, actor, action, resType, resID, ip, details)
}
