package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/nodes"
	"github.com/linux-server-manager/backend/internal/systemmgr"
)

var validServiceRegex = regexp.MustCompile(`^[a-zA-Z0-9_@.-]+\.service$`)

type SystemManagerHandler struct {
	mgr      *systemmgr.SystemManager
	db       *database.DB
	agentHub *nodes.AgentHub
	nodesSvc *nodes.Service
}

func NewSystemManagerHandler(mgr *systemmgr.SystemManager, db *database.DB, agentHub *nodes.AgentHub, nodesSvc *nodes.Service) *SystemManagerHandler {
	return &SystemManagerHandler{
		mgr:      mgr,
		db:       db,
		agentHub: agentHub,
		nodesSvc: nodesSvc,
	}
}

func (h *SystemManagerHandler) getRemoteNode(r *http.Request) (*nodes.Node, bool, error) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		nodeID = r.Header.Get("X-Node-ID")
	}
	if nodeID == "" || nodeID == "local-node" {
		return nil, false, nil
	}
	if h.nodesSvc == nil || h.agentHub == nil {
		return nil, true, errors.New("multi-server não inicializado")
	}
	node, err := h.nodesSvc.GetNode(nodeID)
	if err != nil {
		return nil, true, nodes.ErrNodeNotFound
	}
	if node.Status == nodes.StatusRevoked {
		return nil, true, nodes.ErrNodeRevoked
	}
	if !h.agentHub.IsNodeOnline(nodeID) {
		return nil, true, nodes.ErrNodeOffline
	}
	return node, true, nil
}

func (h *SystemManagerHandler) handleRemoteError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	if errors.Is(err, nodes.ErrNodeNotFound) {
		w.WriteHeader(http.StatusNotFound)
	} else if errors.Is(err, nodes.ErrNodeRevoked) {
		w.WriteHeader(http.StatusForbidden)
	} else if errors.Is(err, nodes.ErrNodeOffline) {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (h *SystemManagerHandler) ListProcesses(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		payload := map[string]interface{}{
			"operator_username": user.Username,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "system.processes", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	procs, err := h.mgr.ListProcesses(user)
	if err != nil {
		http.Error(w, `{"error":"falha ao listar processos"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(procs)
}

func (h *SystemManagerHandler) KillProcess(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "sessão não autenticada"})
		return
	}

	var req systemmgr.KillProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	if req.PID <= 1 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "não é permitido finalizar processo com PID <= 1 ou processos de sistema"})
		return
	}

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		agentToken, err := h.nodesSvc.GetNodeAgentToken(remoteNode.ID)
		if err != nil {
			h.handleRemoteError(w, err)
			return
		}
		ticket, err := systemmgr.CreateProcessKillTicket(remoteNode.ID, agentToken, user.UID, user.Username, user.IsAdmin, req.PID, req.Signal)
		if err != nil {
			http.Error(w, `{"error":"falha ao gerar ticket criptográfico de autorização"}`, http.StatusInternalServerError)
			return
		}
		payload := map[string]interface{}{
			"operator_username": user.Username,
			"operator_uid":      user.UID,
			"is_admin":          user.IsAdmin,
			"pid":               req.PID,
			"signal":            req.Signal,
			"ticket":            ticket,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "system.process_kill", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "kill_process", "process", strconv.Itoa(req.PID), r.RemoteAddr, fmt.Sprintf("Processo finalizado no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	if err := h.mgr.KillProcess(user, req.PID, req.Signal); err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == systemmgr.ErrUnauthorizedProcessAction {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "kill_process", "process", strconv.Itoa(req.PID), r.RemoteAddr, "Processo finalizado")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "processo finalizado com sucesso"})
}

func (h *SystemManagerHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		payload := map[string]interface{}{
			"operator_username": user.Username,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "system.services", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	services, err := h.mgr.ListServices(user)
	if err != nil {
		http.Error(w, `{"error":"falha ao listar serviços"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(services)
}

func (h *SystemManagerHandler) ControlService(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req systemmgr.ServiceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		payload := map[string]interface{}{
			"operator_username": user.Username,
			"service_name":      req.Name,
			"action":            req.Action,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "system.service_action", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "control_service", "service", req.Name+":"+req.Action, r.RemoteAddr, fmt.Sprintf("Ação em serviço no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	if err := h.mgr.ControlService(user, req.Name, req.Action); err != nil {
		w.Header().Set("Content-Type", "application/json")
		if err == systemmgr.ErrUnauthorizedServiceAction {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	h.logAudit(user.Username, "control_service", "service", req.Name+":"+req.Action, r.RemoteAddr, "Ação em serviço systemd")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "ação executada com sucesso"})
}

func (h *SystemManagerHandler) GetServiceLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := auth.UserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "sessão não autenticada"})
		return
	}

	// Regra Crítica de Segurança: consulta a logs de serviço exige permissão de administrador
	if !user.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "acesso restrito a administradores autorizados"})
		return
	}

	serviceName := strings.TrimSpace(r.URL.Query().Get("name"))
	if serviceName == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "parâmetro 'name' é obrigatório"})
		return
	}
	if !strings.HasSuffix(serviceName, ".service") {
		serviceName += ".service"
	}
	if !validServiceRegex.MatchString(serviceName) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "nome do serviço contém caracteres inválidos"})
		return
	}

	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		val, err := strconv.Atoi(linesStr)
		if err != nil || val <= 0 || val > 500 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "parâmetro 'lines' deve ser um número inteiro entre 1 e 500"})
			return
		}
		lines = val
	}

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		payload := map[string]interface{}{
			"operator_username": user.Username,
			"is_admin":          true,
			"service_name":      serviceName,
			"lines":             strconv.Itoa(lines),
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		respBytes, cmdErr := h.agentHub.SendCommand(ctx, remoteNode.ID, "system.service_logs", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}

		// Limita o tamanho máximo de resposta a 512 KB
		if len(respBytes) > 512*1024 {
			respBytes = respBytes[:512*1024]
		}

		h.logAudit(user.Username, "view_service_logs", "service", serviceName, r.RemoteAddr, fmt.Sprintf("Consulta de logs no nó remoto %s", remoteNode.ID))
		_, _ = w.Write(respBytes)
		return
	}

	logs, err := h.mgr.GetServiceLogs(user, serviceName, lines)
	if err != nil {
		if errors.Is(err, systemmgr.ErrUnauthorizedServiceAction) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "acesso restrito a administradores autorizados"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "falha ao consultar logs do serviço"})
		return
	}

	if len(logs) > 512*1024 {
		logs = logs[:512*1024] + "\n[...truncado por exceder 512KB...]"
	}

	h.logAudit(user.Username, "view_service_logs", "service", serviceName, r.RemoteAddr, "Consulta de logs de serviço local")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"service": serviceName,
		"logs":    logs,
	})
}

func (h *SystemManagerHandler) logAudit(actor, action, resType, resID, ip, details string) {
	if h.db == nil {
		return
	}
	query := `INSERT INTO audit_logs (actor, action, resource_type, resource_id, ip_address, details) VALUES (?, ?, ?, ?, ?, ?)`
	_, _ = h.db.Exec(query, actor, action, resType, resID, ip, details)
}
