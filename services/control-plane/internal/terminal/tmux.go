package terminal

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linux-server-manager/backend/internal/auth"
)

type TmuxService struct {
	mu           sync.RWMutex
	devSessions  map[string]map[string]*PersistentSession // user -> sessionName -> session
	hasTmuxBinary bool
}

func NewTmuxService() *TmuxService {
	_, err := exec.LookPath("tmux")
	s := &TmuxService{
		devSessions:  make(map[string]map[string]*PersistentSession),
		hasTmuxBinary: err == nil,
	}

	// Pré-carrega sessão de demonstração para ambiente de desenvolvimento se não houver tmux
	if !s.hasTmuxBinary {
		s.devSessions["admin"] = map[string]*PersistentSession{
			"valheim-server": {
				Name:         "valheim-server",
				ID:           "s-01",
				WindowsCount: 1,
				CreatedAt:    time.Now().Add(-2 * time.Hour),
				IsAttached:   false,
				IsGameServer: true,
				GameName:     "Valheim Dedicated Server",
			},
		}
	}

	return s
}

// ListSessions lista sessões isoladas pertencentes ao usuário autenticado
func (s *TmuxService) ListSessions(user *auth.LinuxUserIdentity) ([]PersistentSession, error) {
	if user == nil {
		return nil, fmt.Errorf("usuário não autenticado")
	}

	if s.hasTmuxBinary {
		// Executa tmux com socket dedicado por usuário: tmux -L lsm-<username> ls
		socketName := fmt.Sprintf("lsm-%s", user.Username)
		// #nosec G204 -- socketName derivado de usuário autenticado
		cmd := exec.Command("tmux", "-L", socketName, "list-sessions", "-F", "#{session_name}:#{session_windows}:#{session_created}:#{session_attached}")
		output, err := cmd.Output()
		if err != nil {
			// Sem sessões ativas
			return []PersistentSession{}, nil
		}

		var list []PersistentSession
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, ":")
			if len(parts) >= 4 {
				name := parts[0]
				windows, _ := strconv.Atoi(parts[1])
				createdUnix, _ := strconv.ParseInt(parts[2], 10, 64)
				attached := parts[3] == "1"

				list = append(list, PersistentSession{
					Name:         name,
					ID:           name,
					WindowsCount: windows,
					CreatedAt:    time.Unix(createdUnix, 0).UTC(),
					IsAttached:   attached,
					IsGameServer: strings.HasPrefix(name, "game-") || strings.Contains(name, "-server"),
				})
			}
		}
		return list, nil
	}

	// Fallback de desenvolvimento em memória
	s.mu.RLock()
	defer s.mu.RUnlock()

	userSessions := s.devSessions[user.Username]
	var list []PersistentSession
	for _, sess := range userSessions {
		list = append(list, *sess)
	}
	return list, nil
}

// CreateSession cria nova sessão persistente desanexada
func (s *TmuxService) CreateSession(user *auth.LinuxUserIdentity, name, gameName string) (*PersistentSession, error) {
	if user == nil {
		return nil, fmt.Errorf("usuário não autenticado")
	}
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("nome da sessão não pode ser vazio")
	}

	// Validação de caracteres no nome da sessão
	for _, r := range cleanName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return nil, fmt.Errorf("o nome da sessão contém caracteres inválidos. Utilize apenas letras, números, hífen e sublinhado")
		}
	}

	if s.hasTmuxBinary {
		socketName := fmt.Sprintf("lsm-%s", user.Username)
		// #nosec G204 -- socketName derivado de usuário autenticado; cleanName validado contra caracteres inválidos
		cmd := exec.Command("tmux", "-L", socketName, "new-session", "-d", "-s", cleanName)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("falha ao criar sessão tmux: %w", err)
		}
	}

	sess := &PersistentSession{
		Name:         cleanName,
		ID:           cleanName,
		WindowsCount: 1,
		CreatedAt:    time.Now().UTC(),
		IsAttached:   false,
		IsGameServer: gameName != "" || strings.HasPrefix(cleanName, "game-"),
		GameName:     gameName,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.devSessions[user.Username]; !ok {
		s.devSessions[user.Username] = make(map[string]*PersistentSession)
	}
	s.devSessions[user.Username][cleanName] = sess

	return sess, nil
}

// KillSession encerra uma sessão persistente mediante autorização
func (s *TmuxService) KillSession(user *auth.LinuxUserIdentity, name string) error {
	if user == nil {
		return fmt.Errorf("usuário não autenticado")
	}

	if s.hasTmuxBinary {
		socketName := fmt.Sprintf("lsm-%s", user.Username)
		// #nosec G204 -- socketName derivado de usuário autenticado; name validado pelo chamador
		cmd := exec.Command("tmux", "-L", socketName, "kill-session", "-t", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("falha ao encerrar sessão tmux: %w", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if userSessions, ok := s.devSessions[user.Username]; ok {
		delete(userSessions, name)
	}

	return nil
}
