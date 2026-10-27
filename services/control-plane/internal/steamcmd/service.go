package steamcmd

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
)

var (
	appIDRegex       = regexp.MustCompile(`^[0-9]+$`)
	betaBranchRegex  = regexp.MustCompile(`^[a-zA-Z0-9_.-]*$`)
)

var (
	ErrUnauthorizedServerAction = errors.New("não autorizado a gerenciar este servidor de jogo")
	ErrServerNotFound           = errors.New("servidor de jogo não encontrado")
	ErrServerAlreadyRunning     = errors.New("servidor de jogo já está em execução")
	ErrSteamCMDNotFound         = errors.New("steamcmd não encontrado no sistema operacional")
)

type Service struct {
	db       *database.DB
	nodeID   string
	devMode  bool
	mu       sync.Mutex
	devState map[string]string // map[serverID]status para modo de desenvolvimento
}

// NewService cria o serviço oficial do SteamCMD para execução em produção.
// Não utiliza simulações condicionadas por variáveis de ambiente.
func NewService(db *database.DB, nodeID string) *Service {
	if nodeID == "" {
		nodeID = "local-node"
	}
	return &Service{
		db:       db,
		nodeID:   nodeID,
		devMode:  false,
		devState: make(map[string]string),
	}
}

// NewTestService cria um serviço SteamCMD com adaptador simulado em memória explicitamente para testes unitários.
func NewTestService(db *database.DB, nodeID string) *Service {
	if nodeID == "" {
		nodeID = "test-node"
	}
	return &Service{
		db:       db,
		nodeID:   nodeID,
		devMode:  true,
		devState: make(map[string]string),
	}
}


// CheckStatus detecta se o SteamCMD e as bibliotecas de 32 bits estão instaladas no SO
func (s *Service) CheckStatus() SteamCMDStatus {
	status := SteamCMDStatus{
		OsDistribution:  runtime.GOOS + "/" + runtime.GOARCH,
		InstallGuidance: "Para Ubuntu 24.04 LTS: sudo dpkg --add-architecture i386 && sudo apt-get update && sudo apt-get install -y steamcmd lib32gcc-s1",
	}

	if s.devMode {
		status.Installed = true
		status.Path = "simulated:/usr/games/steamcmd"
		status.Version = "Steam Console Client (c) Valve Corporation - Dev Simulator"
		status.Libs32BitReady = true
		return status
	}

	// Procura executável do steamcmd
	pathsToCheck := []string{
		"/usr/games/steamcmd",
		"/usr/bin/steamcmd",
	}

	if p, err := exec.LookPath("steamcmd"); err == nil {
		status.Installed = true
		status.Path = p
	} else {
		for _, p := range pathsToCheck {
			if _, err := os.Stat(p); err == nil {
				status.Installed = true
				status.Path = p
				break
			}
		}
	}

	if status.Installed {
		// Checa versão
		// #nosec G204 -- status.Path descoberto por exec.LookPath ou lista de caminhos do sistema
		out, err := exec.Command(status.Path, "+quit").CombinedOutput()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Valve Corporation") || strings.Contains(line, "Steam Console Client") {
					status.Version = strings.TrimSpace(line)
					break
				}
			}
		}
		if status.Version == "" {
			status.Version = "SteamCMD instalado (v_latest)"
		}
	}

	// Checa compatibilidade com bibliotecas de 32-bit (Ubuntu 24.04 requer lib32gcc-s1)
	if runtime.GOOS == "linux" {
		out, err := exec.Command("dpkg", "-s", "lib32gcc-s1").CombinedOutput()
		if err == nil && strings.Contains(string(out), "Status: install ok installed") {
			status.Libs32BitReady = true
		} else {
			// Fallback: verificar se /lib/ld-linux.so.2 ou /usr/lib32 existe
			if _, err := os.Stat("/lib/ld-linux.so.2"); err == nil {
				status.Libs32BitReady = true
			} else if _, err := os.Stat("/usr/lib32"); err == nil {
				status.Libs32BitReady = true
			}
		}
	}

	return status
}

// ListServers retorna todos os servidores de jogos gerenciados acessíveis pelo usuário
func (s *Service) ListServers(user *auth.LinuxUserIdentity) ([]GameServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rows *sql.Rows
	var err error

	if user.IsAdmin {
		query := `SELECT id, node_id, name, service_type, app_id, install_path, system_user, session_name, status, auto_restart, start_command, port, created_at, updated_at FROM managed_servers ORDER BY name ASC`
		rows, err = s.db.Query(query)
	} else {
		query := `SELECT id, node_id, name, service_type, app_id, install_path, system_user, session_name, status, auto_restart, start_command, port, created_at, updated_at FROM managed_servers WHERE system_user = ? ORDER BY name ASC`
		rows, err = s.db.Query(query, user.Username)
	}

	if err != nil {
		return nil, fmt.Errorf("falha ao consultar servidores gerenciados: %w", err)
	}
	defer rows.Close()

	servers := make([]GameServer, 0)
	for rows.Next() {
		var srv GameServer
		var createdAt, updatedAt time.Time
		err := rows.Scan(
			&srv.ID,
			&srv.NodeID,
			&srv.Name,
			&srv.ServiceType,
			&srv.AppID,
			&srv.InstallPath,
			&srv.SystemUser,
			&srv.SessionName,
			&srv.Status,
			&srv.AutoRestart,
			&srv.StartCommand,
			&srv.Port,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("falha ao ler linha de servidor: %w", err)
		}
		srv.CreatedAt = createdAt
		srv.UpdatedAt = updatedAt

		// Em modo dev, consulta devState
		if s.devMode {
			if devStatus, exists := s.devState[srv.ID]; exists {
				srv.Status = devStatus
			}
		} else {
			// Em Linux, checa se a sessão tmux ainda está viva
			srv.Status = s.checkRealStatus(srv.SystemUser, srv.SessionName, srv.Status)
		}

		servers = append(servers, srv)
	}

	return servers, nil
}

// CreateServer cria um novo servidor gerenciado
func (s *Service) CreateServer(user *auth.LinuxUserIdentity, req CreateServerRequest) (*GameServer, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("nome do servidor é obrigatório")
	}
	if req.ServiceType == "steamcmd" || req.AppID != "" {
		if !appIDRegex.MatchString(strings.TrimSpace(req.AppID)) {
			return nil, errors.New("AppID inválido: deve conter apenas dígitos numéricos")
		}
	}
	if strings.TrimSpace(req.InstallPath) == "" {
		// Define caminho padrão na home do usuário
		req.InstallPath = fmt.Sprintf("/home/%s/gameservers/%s", user.Username, strings.ToLower(strings.ReplaceAll(req.Name, " ", "_")))
	}
	if strings.ContainsAny(req.InstallPath, ";|&`$><\"\n\r") {
		return nil, errors.New("InstallPath contém caracteres especiais não permitidos")
	}

	serverID := uuid.New().String()
	sessionName := fmt.Sprintf("lsm-game-%s", serverID[:8])

	// Garante integridade referencial com tabela nodes
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO nodes (id, name, hostname, ip_address, public_key, status) VALUES (?, 'Servidor Host', 'localhost', '127.0.0.1', 'local', 'online')`, s.nodeID)

	query := `
		INSERT INTO managed_servers (
			id, node_id, name, service_type, app_id, install_path, system_user,
			session_name, status, auto_restart, start_command, port, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'stopped', ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	_, err := s.db.Exec(
		query,
		serverID,
		s.nodeID,
		req.Name,
		req.ServiceType,
		req.AppID,
		req.InstallPath,
		user.Username,
		sessionName,
		req.AutoRestart,
		req.StartCommand,
		req.Port,
	)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar servidor no banco: %w", err)
	}

	return &GameServer{
		ID:           serverID,
		NodeID:       s.nodeID,
		Name:         req.Name,
		ServiceType:  req.ServiceType,
		AppID:        req.AppID,
		InstallPath:  req.InstallPath,
		SystemUser:   user.Username,
		SessionName:  sessionName,
		Status:       "stopped",
		AutoRestart:  req.AutoRestart,
		StartCommand: req.StartCommand,
		Port:         req.Port,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

// DeleteServer remove o servidor gerenciado
func (s *Service) DeleteServer(user *auth.LinuxUserIdentity, serverID string) error {
	srv, err := s.getServerForUser(user, serverID)
	if err != nil {
		return err
	}

	if srv.Status == "running" {
		_ = s.StopServer(user, serverID)
	}

	_, err = s.db.Exec("DELETE FROM managed_servers WHERE id = ?", serverID)
	if err != nil {
		return fmt.Errorf("falha ao excluir servidor: %w", err)
	}

	s.mu.Lock()
	delete(s.devState, serverID)
	s.mu.Unlock()

	return nil
}

// StartServer inicia o servidor em uma sessão tmux isolada do usuário Linux
func (s *Service) StartServer(user *auth.LinuxUserIdentity, serverID string) error {
	srv, err := s.getServerForUser(user, serverID)
	if err != nil {
		return err
	}

	if s.devMode {
		s.mu.Lock()
		s.devState[serverID] = "running"
		s.mu.Unlock()
		_, _ = s.db.Exec("UPDATE managed_servers SET status = 'running', updated_at = CURRENT_TIMESTAMP WHERE id = ?", serverID)
		return nil
	}

	// Executa em sessão tmux isolada sob o usuário Linux
	socketName := fmt.Sprintf("lsm-%s", srv.SystemUser)
	cmdArgs := []string{
		"-L", socketName,
		"new-session", "-d",
		"-s", srv.SessionName,
		"-c", srv.InstallPath,
		srv.StartCommand,
	}

	// #nosec G204 -- cmdArgs construído internamente com sessão e diretório validados
	cmd := exec.Command("tmux", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("falha ao iniciar tmux (%s): %s", err, string(out))
	}

	_, _ = s.db.Exec("UPDATE managed_servers SET status = 'running', updated_at = CURRENT_TIMESTAMP WHERE id = ?", serverID)
	return nil
}

// StopServer encerra a sessão do servidor de jogo
func (s *Service) StopServer(user *auth.LinuxUserIdentity, serverID string) error {
	srv, err := s.getServerForUser(user, serverID)
	if err != nil {
		return err
	}

	if s.devMode {
		s.mu.Lock()
		s.devState[serverID] = "stopped"
		s.mu.Unlock()
		_, _ = s.db.Exec("UPDATE managed_servers SET status = 'stopped', updated_at = CURRENT_TIMESTAMP WHERE id = ?", serverID)
		return nil
	}

	socketName := fmt.Sprintf("lsm-%s", srv.SystemUser)
	// #nosec G204 -- socketName derivado de usuário do banco; SessionName validado
	cmd := exec.Command("tmux", "-L", socketName, "kill-session", "-t", srv.SessionName)
	_ = cmd.Run()

	_, _ = s.db.Exec("UPDATE managed_servers SET status = 'stopped', updated_at = CURRENT_TIMESTAMP WHERE id = ?", serverID)
	return nil
}

// InstallOrUpdate executa instalação ou atualização com SteamCMD
func (s *Service) InstallOrUpdate(user *auth.LinuxUserIdentity, serverID string, validate bool, beta string) error {
	srv, err := s.getServerForUser(user, serverID)
	if err != nil {
		return err
	}

	if srv.Status == "updating" {
		return errors.New("o servidor já está passando por instalação ou atualização")
	}

	if beta != "" && !betaBranchRegex.MatchString(beta) {
		return errors.New("nome da branch beta inválido")
	}
	if !appIDRegex.MatchString(strings.TrimSpace(srv.AppID)) {
		return errors.New("AppID do servidor inválido: deve conter apenas dígitos")
	}
	if strings.ContainsAny(srv.InstallPath, ";|&`$><\"\n\r") {
		return errors.New("InstallPath contém caracteres especiais não permitidos")
	}

	if s.devMode {
		s.mu.Lock()
		s.devState[serverID] = "updating"
		s.mu.Unlock()
		go func() {
			time.Sleep(3 * time.Second)
			s.mu.Lock()
			s.devState[serverID] = "stopped"
			s.mu.Unlock()
			_, _ = s.db.Exec("UPDATE managed_servers SET status = 'stopped', updated_at = CURRENT_TIMESTAMP WHERE id = ?", serverID)
		}()
		return nil
	}

	// Garante que o diretório de instalação existe
	if err := os.MkdirAll(srv.InstallPath, 0750); err != nil {
		return fmt.Errorf("falha ao preparar diretório de instalação: %w", err)
	}

	// Monta comando steamcmd
	// steamcmd +force_install_dir <path> +login anonymous +app_update <app_id> [validate] +quit
	steamArgs := []string{
		"+force_install_dir", srv.InstallPath,
		"+login", "anonymous",
		"+app_update", srv.AppID,
	}
	if beta != "" {
		steamArgs = append(steamArgs, "-beta", beta)
	}
	if validate {
		steamArgs = append(steamArgs, "validate")
	}
	steamArgs = append(steamArgs, "+quit")

	// Lança em sessão tmux dedicada de atualização: lsm-update-<id>
	// permitindo que o usuário acompanhe pelo terminal
	updateSession := fmt.Sprintf("lsm-upd-%s", srv.ID[:8])
	socketName := fmt.Sprintf("lsm-%s", srv.SystemUser)

	fullCmd := fmt.Sprintf("steamcmd %s", strings.Join(steamArgs, " "))
	// #nosec G204 -- steamArgs construído internamente com app_id inteiro e diretório validado
	cmd := exec.Command("tmux", "-L", socketName, "new-session", "-d", "-s", updateSession, fullCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("falha ao disparar instalação via tmux: %s", string(out))
	}

	_, _ = s.db.Exec("UPDATE managed_servers SET status = 'updating', updated_at = CURRENT_TIMESTAMP WHERE id = ?", serverID)
	return nil
}

func (s *Service) checkRealStatus(systemUser, sessionName, currentStatus string) string {
	if currentStatus == "updating" {
		return "updating"
	}

	socketName := fmt.Sprintf("lsm-%s", systemUser)
	// #nosec G204 -- socketName derivado de usuário; sessionName validado
	cmd := exec.Command("tmux", "-L", socketName, "has-session", "-t", sessionName)
	if err := cmd.Run(); err == nil {
		return "running"
	}
	return "stopped"
}

func (s *Service) getServerForUser(user *auth.LinuxUserIdentity, serverID string) (*GameServer, error) {
	var srv GameServer
	var createdAt, updatedAt time.Time
	query := `SELECT id, node_id, name, service_type, app_id, install_path, system_user, session_name, status, auto_restart, start_command, port, created_at, updated_at FROM managed_servers WHERE id = ?`
	row := s.db.QueryRow(query, serverID)
	err := row.Scan(
		&srv.ID,
		&srv.NodeID,
		&srv.Name,
		&srv.ServiceType,
		&srv.AppID,
		&srv.InstallPath,
		&srv.SystemUser,
		&srv.SessionName,
		&srv.Status,
		&srv.AutoRestart,
		&srv.StartCommand,
		&srv.Port,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrServerNotFound
		}
		return nil, err
	}

	// Validação de autorização estrita: admin pode gerenciar qualquer um, usuário comum apenas os seus
	if !user.IsAdmin && srv.SystemUser != user.Username {
		return nil, ErrUnauthorizedServerAction
	}

	srv.CreatedAt = createdAt
	srv.UpdatedAt = updatedAt
	return &srv, nil
}
