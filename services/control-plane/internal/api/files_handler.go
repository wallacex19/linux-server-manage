package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/files"
	"github.com/linux-server-manager/backend/internal/nodes"
)

type FilesHandler struct {
	service  *files.FileService
	db       *database.DB
	agentHub *nodes.AgentHub
	nodesSvc *nodes.Service
}

func NewFilesHandler(service *files.FileService, db *database.DB, agentHub *nodes.AgentHub, nodesSvc *nodes.Service) *FilesHandler {
	return &FilesHandler{
		service:  service,
		db:       db,
		agentHub: agentHub,
		nodesSvc: nodesSvc,
	}
}

func (h *FilesHandler) getRemoteNode(r *http.Request) (*nodes.Node, bool, error) {
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

func (h *FilesHandler) handleRemoteError(w http.ResponseWriter, err error) {
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

func (h *FilesHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	path := r.URL.Query().Get("path")

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		payload := map[string]interface{}{
			"operator_username": user.Username,
			"path":              path,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "files.list", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	listing, err := h.service.ListDirectory(user, path)
	if err != nil {
		h.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listing)
}

func (h *FilesHandler) Read(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	path := r.URL.Query().Get("path")

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		payload := map[string]interface{}{
			"operator_username": user.Username,
			"path":              path,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "files.read", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "file_read", "file", path, r.RemoteAddr, fmt.Sprintf("Arquivo lido no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	fileContent, err := h.service.ReadFile(user, path)
	if err != nil {
		h.writeError(w, err)
		return
	}

	h.logAudit(user.Username, "file_read", "file", path, r.RemoteAddr, "Arquivo lido no editor")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fileContent)
}

type SaveFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (h *FilesHandler) Save(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req SaveFileRequest
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
			"path":              req.Path,
			"content":           req.Content,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "files.save", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "file_save", "file", req.Path, r.RemoteAddr, fmt.Sprintf("Arquivo salvo no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	if err := h.service.WriteFile(user, req.Path, req.Content); err != nil {
		h.writeError(w, err)
		return
	}

	h.logAudit(user.Username, "file_save", "file", req.Path, r.RemoteAddr, "Arquivo editado e salvo")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "arquivo salvo com sucesso"})
}

type PathRequest struct {
	Path string `json:"path"`
}

func (h *FilesHandler) Mkdir(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req PathRequest
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
			"path":              req.Path,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "files.mkdir", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "mkdir", "directory", req.Path, r.RemoteAddr, fmt.Sprintf("Diretório criado no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	if err := h.service.CreateDirectory(user, req.Path); err != nil {
		h.writeError(w, err)
		return
	}

	h.logAudit(user.Username, "mkdir", "directory", req.Path, r.RemoteAddr, "Diretório criado")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "diretório criado"})
}

func (h *FilesHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req PathRequest
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
			"path":              req.Path,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "files.create", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "file_create", "file", req.Path, r.RemoteAddr, fmt.Sprintf("Arquivo criado no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	if err := h.service.CreateFile(user, req.Path); err != nil {
		h.writeError(w, err)
		return
	}

	h.logAudit(user.Username, "file_create", "file", req.Path, r.RemoteAddr, "Arquivo vazio criado")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "arquivo criado"})
}

type RenameRequest struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
}

func (h *FilesHandler) Rename(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req RenameRequest
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
			"old_path":          req.OldPath,
			"new_path":          req.NewPath,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "files.rename", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "file_rename", "file", req.OldPath+" -> "+req.NewPath, r.RemoteAddr, fmt.Sprintf("Arquivo renomeado no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	if err := h.service.Rename(user, req.OldPath, req.NewPath); err != nil {
		h.writeError(w, err)
		return
	}

	h.logAudit(user.Username, "file_rename", "file", req.OldPath+" -> "+req.NewPath, r.RemoteAddr, "Arquivo/pasta renomeado")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "renomeado com sucesso"})
}

func (h *FilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	path := r.URL.Query().Get("path")

	remoteNode, isRemote, err := h.getRemoteNode(r)
	if err != nil {
		h.handleRemoteError(w, err)
		return
	}

	if isRemote {
		payload := map[string]interface{}{
			"operator_username": user.Username,
			"path":              path,
		}
		respBytes, cmdErr := h.agentHub.SendCommand(r.Context(), remoteNode.ID, "files.delete", payload)
		if cmdErr != nil {
			h.handleRemoteError(w, cmdErr)
			return
		}
		h.logAudit(user.Username, "file_delete", "file", path, r.RemoteAddr, fmt.Sprintf("Arquivo excluído no nó remoto %s", remoteNode.ID))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBytes)
		return
	}

	if err := h.service.Delete(user, path); err != nil {
		h.writeError(w, err)
		return
	}

	h.logAudit(user.Username, "file_delete", "file", path, r.RemoteAddr, "Arquivo/pasta excluído")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "excluído com sucesso"})
}

func (h *FilesHandler) Download(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	path := r.URL.Query().Get("path")

	targetPath, err := h.service.ResolveAndValidatePath(user, path)
	if err != nil {
		h.writeError(w, err)
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		http.Error(w, `{"error":"arquivo não encontrado ou é um diretório"}`, http.StatusNotFound)
		return
	}

	h.logAudit(user.Username, "file_download", "file", path, r.RemoteAddr, "Download de arquivo")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(targetPath)+"\"")
	http.ServeFile(w, r, targetPath)
}

func (h *FilesHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	// Limite de 100 MB para upload
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		http.Error(w, `{"error":"tamanho do arquivo excede o limite permitido de 100 MB"}`, http.StatusBadRequest)
		return
	}

	targetDir := r.FormValue("target_dir")
	resolvedDir, err := h.service.ResolveAndValidatePath(user, targetDir)
	if err != nil {
		h.writeError(w, err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"campo 'file' ausente na requisição"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	safeFileName := filepath.Base(header.Filename)
	destination := filepath.Join(resolvedDir, safeFileName)

	// #nosec G304 -- destination construído com resolvedDir validado e filepath.Base do arquivo
	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		h.writeError(w, err)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, `{"error":"falha ao gravar arquivo transferido"}`, http.StatusInternalServerError)
		return
	}

	h.logAudit(user.Username, "file_upload", "file", destination, r.RemoteAddr, "Upload de arquivo concluído")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "upload realizado com sucesso", "path": destination})
}

func (h *FilesHandler) writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	switch err {
	case files.ErrAccessDenied, files.ErrPathTraversal:
		w.WriteHeader(http.StatusForbidden)
	case files.ErrFileNotFound:
		w.WriteHeader(http.StatusNotFound)
	case files.ErrBinaryFile, files.ErrCriticalTarget, files.ErrTargetExists:
		w.WriteHeader(http.StatusBadRequest)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (h *FilesHandler) logAudit(actor, action, resType, resID, ip, details string) {
	if h.db == nil {
		return
	}
	query := `INSERT INTO audit_logs (actor, action, resource_type, resource_id, ip_address, details) VALUES (?, ?, ?, ?, ?, ?)`
	_, _ = h.db.Exec(query, actor, action, resType, resID, ip, details)
}
