package systemmgr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/linux-server-manager/backend/internal/auth"
)

var (
	ErrUnauthorizedServiceAction = errors.New("permissão negada: apenas administradores autorizados podem gerenciar serviços systemd")
	ErrUnauthorizedProcessAction = errors.New("permissão negada: você não pode encerrar processos de outros usuários ou do sistema")
	ErrInvalidAction             = errors.New("ação de serviço inválida. Permitidos: start, stop, restart")
)

type SystemManager struct {
	mu           sync.RWMutex
	devServices  []ServiceInfo
	devProcesses []ProcessInfo
	hasSystemctl bool
	isTest       bool
}

// NewSystemManager cria o gerenciador oficial para execução em produção.
// No Linux, delega comandos de serviços ao systemctl do host sem simulações ativadas por variáveis de ambiente.
// O runtime oficial de produção não possui dados simulados e não faz fallback inseguro.
func NewSystemManager() *SystemManager {
	_, err := exec.LookPath("systemctl")
	return &SystemManager{
		hasSystemctl: err == nil && runtime.GOOS == "linux",
		isTest:       false,
	}
}

// NewTestSystemManager cria um gerenciador simulado em memória explicitamente para testes unitários.
func NewTestSystemManager() *SystemManager {
	mgr := &SystemManager{
		hasSystemctl: false,
		isTest:       true,
	}
	mgr.initDevData()
	return mgr
}

func (mgr *SystemManager) initDevData() {
	mgr.devServices = []ServiceInfo{
		{
			Name:        "lsm-control-plane.service",
			Description: "Linux Server Manager - Control Plane API",
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "running",
			CanControl:  true,
		},
		{
			Name:        "lsm-agent.service",
			Description: "Linux Server Manager - Server Agent Node",
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "running",
			CanControl:  true,
		},
		{
			Name:        "caddy.service",
			Description: "Caddy Web Server with TLS and Reverse Proxy",
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "running",
			CanControl:  true,
		},
		{
			Name:        "valheim-server.service",
			Description: "Valheim Dedicated Game Server",
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "running",
			CanControl:  true,
		},
		{
			Name:        "ssh.service",
			Description: "OpenBSD Secure Shell server",
			LoadState:   "loaded",
			ActiveState: "active",
			SubState:    "running",
			CanControl:  true,
		},
	}

	mgr.devProcesses = []ProcessInfo{
		{
			PID:           1,
			User:          "root",
			UID:           0,
			Name:          "systemd",
			Command:       "/sbin/init",
			CPUPercent:    0.1,
			MemoryBytes:   18 * 1024 * 1024,
			MemoryPercent: 0.2,
			State:         "Sleeping",
		},
		{
			PID:           512,
			User:          "caddy",
			UID:           998,
			Name:          "caddy",
			Command:       "/usr/bin/caddy run --config /etc/caddy/Caddyfile",
			CPUPercent:    0.8,
			MemoryBytes:   42 * 1024 * 1024,
			MemoryPercent: 0.5,
			State:         "Running",
		},
		{
			PID:           1024,
			User:          "admin",
			UID:           1000,
			Name:          "lsm-server",
			Command:       "/usr/local/bin/lsm-control-plane",
			CPUPercent:    1.2,
			MemoryBytes:   56 * 1024 * 1024,
			MemoryPercent: 0.7,
			State:         "Running",
		},
		{
			PID:           1450,
			User:          "operador",
			UID:           1001,
			Name:          "tmux",
			Command:       "tmux -L lsm-operador attach",
			CPUPercent:    0.4,
			MemoryBytes:   12 * 1024 * 1024,
			MemoryPercent: 0.1,
			State:         "Sleeping",
		},
		{
			PID:           2048,
			User:          "operador",
			UID:           1001,
			Name:          "valheim_server",
			Command:       "./valheim_server.x86_64 -name 'LSM Server' -port 2456",
			CPUPercent:    14.5,
			MemoryBytes:   1420 * 1024 * 1024,
			MemoryPercent: 12.4,
			State:         "Running",
		},
	}
}

// ListProcesses lista os processos com informações de consumo
func (m *SystemManager) ListProcesses(user *auth.LinuxUserIdentity) ([]ProcessInfo, error) {
	if user == nil {
		return nil, errors.New("não autenticado")
	}

	if !m.isTest {
		return nil, errors.New("gerenciamento direto de processos do host desabilitado; conecte via nó de agente")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Retorna lista com identificação de permissão
	list := make([]ProcessInfo, len(m.devProcesses))
	copy(list, m.devProcesses)
	return list, nil
}

// KillProcess encerra um processo garantindo a regra de isolamento de usuário
func (m *SystemManager) KillProcess(user *auth.LinuxUserIdentity, pid int, signal string) error {
	if user == nil {
		return errors.New("não autenticado")
	}

	if !m.isTest {
		return errors.New("encerramento de processos locais desabilitado; utilize o agente do nó com autorização verificável")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var targetProc *ProcessInfo
	var targetIdx int
	for idx, p := range m.devProcesses {
		if p.PID == pid {
			targetProc = &m.devProcesses[idx]
			targetIdx = idx
			break
		}
	}

	if targetProc == nil {
		return fmt.Errorf("processo PID %d não encontrado", pid)
	}

	// Regra de Segurança: Usuário não-admin NÃO pode encerrar processos de outros usuários ou root
	if !user.IsAdmin && targetProc.UID != user.UID {
		return ErrUnauthorizedProcessAction
	}

	// Remove da lista
	m.devProcesses = append(m.devProcesses[:targetIdx], m.devProcesses[targetIdx+1:]...)
	return nil
}

// ListServices lista unidades systemd
func (m *SystemManager) ListServices(user *auth.LinuxUserIdentity) ([]ServiceInfo, error) {
	if user == nil {
		return nil, errors.New("não autenticado")
	}

	if m.hasSystemctl {
		cmd := exec.Command("systemctl", "list-units", "--type=service", "--no-legend", "--no-pager")
		output, err := cmd.Output()
		if err == nil {
			var list []ServiceInfo
			scanner := bufio.NewScanner(strings.NewReader(string(output)))
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) >= 4 {
					name := fields[0]
					load := fields[1]
					active := fields[2]
					sub := fields[3]
					desc := strings.Join(fields[4:], " ")

					list = append(list, ServiceInfo{
						Name:        name,
						Description: desc,
						LoadState:   load,
						ActiveState: active,
						SubState:    sub,
						CanControl:  user.IsAdmin,
					})
				}
			}
			return list, nil
		}
	}

	if !m.isTest {
		return nil, errors.New("systemctl não disponível neste sistema")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]ServiceInfo, len(m.devServices))
	for i, s := range m.devServices {
		item := s
		item.CanControl = user.IsAdmin
		list[i] = item
	}
	return list, nil
}

// ControlService executa start, stop ou restart de um serviço
func (m *SystemManager) ControlService(user *auth.LinuxUserIdentity, name, action string) error {
	if user == nil {
		return errors.New("não autenticado")
	}

	// Regra de Segurança: apenas administradores autorizados podem gerenciar serviços
	if !user.IsAdmin {
		return ErrUnauthorizedServiceAction
	}

	act := strings.ToLower(strings.TrimSpace(action))
	if act != "start" && act != "stop" && act != "restart" {
		return ErrInvalidAction
	}

	if m.hasSystemctl {
		// #nosec G204 -- act validado contra whitelist estrita; name por IsValidServiceName
		cmd := exec.Command("systemctl", act, name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("falha ao executar systemctl %s %s: %w", act, name, err)
		}
		return nil
	}

	if !m.isTest {
		return errors.New("systemctl não disponível neste sistema")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for idx, s := range m.devServices {
		if s.Name == name {
			if act == "stop" {
				m.devServices[idx].ActiveState = "inactive"
				m.devServices[idx].SubState = "dead"
			} else {
				m.devServices[idx].ActiveState = "active"
				m.devServices[idx].SubState = "running"
			}
			return nil
		}
	}

	return fmt.Errorf("serviço %s não encontrado", name)
}

// GetServiceLogs retorna os logs recentes de um serviço
func (m *SystemManager) GetServiceLogs(user *auth.LinuxUserIdentity, name string, lines int) (string, error) {
	if user == nil {
		return "", errors.New("não autenticado")
	}

	// Regra de Segurança: logs de serviços do sistema são restritos a administradores autorizados
	if !user.IsAdmin {
		return "", ErrUnauthorizedServiceAction
	}

	if lines <= 0 || lines > 500 {
		lines = 100
	}

	if m.hasSystemctl {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// #nosec G204 -- name validado previamente por IsValidServiceName; lines é inteiro delimitado (1-500)
		cmd := exec.CommandContext(ctx, "journalctl", "-u", name, fmt.Sprintf("-n%d", lines), "--no-pager")
		out, err := cmd.Output()
		if err == nil {
			return string(out), nil
		}
	}

	if !m.isTest {
		return "", errors.New("journalctl não disponível neste sistema")
	}

	// Simulação de logs para testes unitários
	return fmt.Sprintf(`[systemd] Started %s.
[info] Service active and responding normally.
[healthcheck] All health checks passed successfully.
[audit] Running under dedicated system user.`, name), nil
}
