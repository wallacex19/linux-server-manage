package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/steamcmd"
)

type SteamCMDHandler struct {
	svc *steamcmd.Service
	db  *database.DB
}

func NewSteamCMDHandler(svc *steamcmd.Service, db *database.DB) *SteamCMDHandler {
	return &SteamCMDHandler{
		svc: svc,
		db:  db,
	}
}

func (h *SteamCMDHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.svc.CheckStatus()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (h *SteamCMDHandler) GetPresets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(steamcmd.PopularPresets)
}

func (h *SteamCMDHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	servers, err := h.svc.ListServers(user)
	if err != nil {
		http.Error(w, `{"error":"falha ao listar servidores"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(servers)
}

func (h *SteamCMDHandler) CreateServer(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req steamcmd.CreateServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	server, err := h.svc.CreateServer(user, req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "create_server", "game_server", server.ID, r.RemoteAddr, "Servidor criado: "+server.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(server)
}

func (h *SteamCMDHandler) DeleteServer(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	serverID := chi.URLParam(r, "id")

	err := h.svc.DeleteServer(user, serverID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == steamcmd.ErrUnauthorizedServerAction {
			w.WriteHeader(http.StatusForbidden)
		} else if err == steamcmd.ErrServerNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "delete_server", "game_server", serverID, r.RemoteAddr, "Servidor excluído")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "servidor excluído com sucesso"})
}

func (h *SteamCMDHandler) StartServer(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	serverID := chi.URLParam(r, "id")

	err := h.svc.StartServer(user, serverID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == steamcmd.ErrUnauthorizedServerAction {
			w.WriteHeader(http.StatusForbidden)
		} else if err == steamcmd.ErrServerNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "start_server", "game_server", serverID, r.RemoteAddr, "Servidor iniciado")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "servidor iniciado com sucesso"})
}

func (h *SteamCMDHandler) StopServer(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	serverID := chi.URLParam(r, "id")

	err := h.svc.StopServer(user, serverID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == steamcmd.ErrUnauthorizedServerAction {
			w.WriteHeader(http.StatusForbidden)
		} else if err == steamcmd.ErrServerNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "stop_server", "game_server", serverID, r.RemoteAddr, "Servidor parado")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "servidor finalizado com sucesso"})
}

func (h *SteamCMDHandler) UpdateServer(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	serverID := chi.URLParam(r, "id")

	var req steamcmd.InstallUpdateRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	err := h.svc.InstallOrUpdate(user, serverID, req.Validate, req.Beta)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == steamcmd.ErrUnauthorizedServerAction {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "update_server", "game_server", serverID, r.RemoteAddr, "Disparado update SteamCMD")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "atualização SteamCMD iniciada em segundo plano"})
}

func (h *SteamCMDHandler) logAudit(actor, action, resType, resID, ip, details string) {
	if h.db == nil {
		return
	}
	query := `INSERT INTO audit_logs (actor, action, resource_type, resource_id, ip_address, details) VALUES (?, ?, ?, ?, ?, ?)`
	_, _ = h.db.Exec(query, actor, action, resType, resID, ip, details)
}
