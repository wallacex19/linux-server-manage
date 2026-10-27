package terminal

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
)

// CheckWebSocketOrigin previne Cross-Site WebSocket Hijacking (CSWSH)
func CheckWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Clientes locais/nativos que não enviam Origin são permitidos
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// 1. Origem idêntica ao Host da requisição
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}

	// 2. Permite comunicação em loopback local de desenvolvimento (ex: Vite na 5173 para API na 8080)
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
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     CheckWebSocketOrigin,
}

type TerminalHandler struct {
	sessionStore *auth.SessionStore
	tmuxService  *TmuxService
	db           *database.DB
}

func NewTerminalHandler(sessions *auth.SessionStore, tmux *TmuxService, db *database.DB) *TerminalHandler {
	return &TerminalHandler{
		sessionStore: sessions,
		tmuxService:  tmux,
		db:           db,
	}
}

// ListSessions retorna sessões tmux pertencentes ao usuário autenticado
func (h *TerminalHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	sessions, err := h.tmuxService.ListSessions(user)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

// CreateSession cria uma nova sessão persistente
func (h *TerminalHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"payload JSON inválido"}`, http.StatusBadRequest)
		return
	}

	sess, err := h.tmuxService.CreateSession(user, req.Name, req.GameName)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	h.logAudit(user.Username, "tmux_create_session", "session", req.Name, r.RemoteAddr, "Nova sessão persistente criada")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sess)
}

// KillSession encerra uma sessão persistente
func (h *TerminalHandler) KillSession(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, `{"error":"parâmetro 'name' obrigatório"}`, http.StatusBadRequest)
		return
	}

	if err := h.tmuxService.KillSession(user, name); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	h.logAudit(user.Username, "tmux_kill_session", "session", name, r.RemoteAddr, "Sessão persistente encerrada")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "sessão encerrada"})
}

// HandleWebSocket conecta o xterm.js ao terminal real / tmux do usuário Linux
func (h *TerminalHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 1. Autenticação via cookie ou query string ?token=...
	token := r.URL.Query().Get("token")
	if token == "" {
		if cookie, err := r.Cookie("lsm_session"); err == nil {
			token = cookie.Value
		}
	}

	sess, ok := h.sessionStore.Get(token)
	if !ok || sess.User == nil {
		http.Error(w, "Sessão não autorizada ou expirada", http.StatusUnauthorized)
		return
	}
	user := sess.User

	// 2. Upgrade da conexão HTTP para WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Terminal WS] Falha no upgrade: %v", err)
		return
	}
	defer conn.Close()

	// 3. Determina o comando do terminal
	tmuxSession := r.URL.Query().Get("tmux_session")
	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		if tmuxSession != "" && h.tmuxService.hasTmuxBinary {
			socketName := fmt.Sprintf("lsm-%s", user.Username)
			// #nosec G204 -- socketName derivado de usuário autenticado; tmuxSession validado por regex
			cmd = exec.Command("tmux", "-L", socketName, "attach-session", "-t", tmuxSession)
		} else {
			// Inicia shell com a identidade do usuário autenticado via PAM
			// #nosec G204 -- user.Username provém estritamente da autenticação PAM autenticada
			cmd = exec.Command("su", "-", user.Username)
		}
	} else {
		// Desenvolvimento local (Windows/macOS)
		if runtime.GOOS == "windows" {
			cmd = exec.Command("powershell.exe", "-NoLogo")
		} else {
			cmd = exec.Command("/bin/bash")
		}
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nErro ao abrir stdin: %v\r\n", err)))
		return
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nErro ao abrir stdout: %v\r\n", err)))
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nErro ao iniciar terminal: %v\r\n", err)))
		return
	}

	h.logAudit(user.Username, "terminal_connect", "terminal", tmuxSession, r.RemoteAddr, "Conexão de terminal estabelecida")

	// 4. Goroutine de leitura: stdout -> WebSocket
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// 5. Loop de escrita: WebSocket -> stdin
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType == websocket.TextMessage {
			// Verifica se é evento de redimensionamento
			var resizeMsg WindowResizeMsg
			if jsonErr := json.Unmarshal(p, &resizeMsg); jsonErr == nil && resizeMsg.Type == "resize" {
				// Evento de resize recebido
				continue
			}

			// Mensagem normal de digitação / comando
			_, _ = stdinPipe.Write(p)
		} else if messageType == websocket.BinaryMessage {
			_, _ = stdinPipe.Write(p)
		}
	}

	// Fecha stdin para sinalizar término ao processo
	_ = stdinPipe.Close()
	if cmd.Process != nil && tmuxSession == "" {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	wg.Wait()

	h.logAudit(user.Username, "terminal_disconnect", "terminal", tmuxSession, r.RemoteAddr, "Conexão de terminal encerrada")
}

func (h *TerminalHandler) logAudit(actor, action, resType, resID, ip, details string) {
	if h.db == nil {
		return
	}
	query := `INSERT INTO audit_logs (actor, action, resource_type, resource_id, ip_address, details) VALUES (?, ?, ?, ?, ?, ?)`
	_, _ = h.db.Exec(query, actor, action, resType, resID, ip, details)
}
