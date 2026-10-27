package system

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
}

var (
	validServiceRegex = regexp.MustCompile(`^[a-zA-Z0-9_@.-]+\.service$`)
)

// IsValidServiceName verifica se o nome do serviço systemd é sintaticamente válido
func IsValidServiceName(name string) bool {
	return validServiceRegex.MatchString(name)
}

// ListServices retorna a lista real de serviços systemd instalados no host Linux
func ListServices() ([]ServiceInfo, error) {
	if runtime.GOOS != "linux" {
		return listServicesFallback()
	}

	cmd := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-pager", "--no-legend")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("falha ao consultar systemctl list-units: %w", err)
	}

	var services []ServiceInfo
	scanner := bufio.NewScanner(strings.NewReader(string(out)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Formato do systemctl: UNIT LOAD ACTIVE SUB DESCRIPTION...
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			unitName := fields[0]
			if strings.HasPrefix(unitName, "●") {
				// Remove indicador de falha se presente no início
				if len(fields) >= 6 {
					fields = fields[1:]
					unitName = fields[0]
				}
			}

			if !strings.HasSuffix(unitName, ".service") {
				continue
			}

			loadState := fields[1]
			activeState := fields[2]
			subState := fields[3]
			desc := strings.Join(fields[4:], " ")

			services = append(services, ServiceInfo{
				Name:        unitName,
				Description: desc,
				LoadState:   loadState,
				ActiveState: activeState,
				SubState:    subState,
			})
		}
	}

	return services, nil
}

func listServicesFallback() ([]ServiceInfo, error) {
	return []ServiceInfo{
		{Name: "ssh.service", Description: "OpenBSD Secure Shell server", LoadState: "loaded", ActiveState: "active", SubState: "running"},
		{Name: "lsm-agent.service", Description: "Linux Server Manager Agent", LoadState: "loaded", ActiveState: "active", SubState: "running"},
	}, nil
}

// ControlService executa uma ação de controle em um serviço systemd aplicando a estratégia de menor privilégio
func ControlService(serviceName, action, helperPath string, isAdmin bool) error {
	if !isAdmin {
		return errors.New("somente administradores podem controlar serviços systemd")
	}

	// 1. Validação estrita da ação
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "start", "stop", "restart", "reload", "status":
		// Ações permitidas
	default:
		return fmt.Errorf("ação '%s' não permitida em serviços systemd", action)
	}

	// 2. Validação e sanitização do nome do serviço
	cleanName := strings.TrimSpace(serviceName)
	if !strings.HasSuffix(cleanName, ".service") {
		cleanName += ".service"
	}
	if !validServiceRegex.MatchString(cleanName) {
		return errors.New("nome do serviço contém caracteres inválidos")
	}

	// 3. Estratégia de Menor Privilégio via Helper Dedicado
	// Se o helper privilegiado com interface restrita estiver instalado, executa via helper
	if helperPath != "" {
		if _, err := os.Stat(helperPath); err == nil {
			var cmd *exec.Cmd
			if os.Geteuid() == 0 {
				// #nosec G204 -- helperPath validado por os.Stat; action por whitelist; cleanName por regex
				cmd = exec.Command(helperPath, "service", action, cleanName)
			} else {
				// #nosec G204 -- argumentos validados contra whitelist e regex canônica
				cmd = exec.Command("sudo", "-n", helperPath, "service", action, cleanName)
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("falha ao executar ação via helper (%w): %s", err, strings.TrimSpace(string(out)))
			}
			return nil
		}
	}

	// 4. Fallback direto se estiver em desenvolvimento ou se o helper não estiver presente
	if runtime.GOOS == "linux" {
		// #nosec G204 -- action validado por whitelist; cleanName por regex
		cmd := exec.Command("systemctl", action, cleanName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("falha no systemctl (%w): %s", err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

// GetServiceLogs lê os logs reais do serviço via journalctl ou helper
func GetServiceLogs(serviceName string, lines int, helperPath string) (string, error) {
	cleanName := strings.TrimSpace(serviceName)
	if !strings.HasSuffix(cleanName, ".service") {
		cleanName += ".service"
	}
	if !validServiceRegex.MatchString(cleanName) {
		return "", errors.New("nome do serviço inválido")
	}

	if lines <= 0 {
		lines = 50
	} else if lines > 500 {
		lines = 500
	}

	// 1. Tenta via helper privilegiado se disponível
	if helperPath != "" && runtime.GOOS == "linux" {
		if _, err := os.Stat(helperPath); err == nil {
			var cmd *exec.Cmd
			if os.Geteuid() == 0 {
				// #nosec G204 -- helperPath validado; cleanName por regex; lines é inteiro limitado
				cmd = exec.Command(helperPath, "journal", cleanName, strconv.Itoa(lines))
			} else {
				// #nosec G204 -- cleanName validado por regex; lines é inteiro limitado
				cmd = exec.Command("sudo", "-n", helperPath, "journal", cleanName, strconv.Itoa(lines))
			}
			out, err := cmd.CombinedOutput()
			if err == nil {
				res := string(out)
				if len(res) > 512*1024 {
					res = res[:512*1024]
				}
				return res, nil
			}
		}
	}

	if runtime.GOOS != "linux" {
		return fmt.Sprintf("-- Logs simulados de %s (Ambiente de desenvolvimento %s) --\nLinha 1: Serviço operacional ativo.\n", cleanName, runtime.GOOS), nil
	}

	// #nosec G204 -- cleanName validado por regex; lines é inteiro limitado (1-500)
	cmd := exec.Command("journalctl", "-u", cleanName, "-n", strconv.Itoa(lines), "--no-pager")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("falha ao consultar journalctl (%w): %s", err, strings.TrimSpace(string(out)))
	}

	res := string(out)
	if len(res) > 512*1024 {
		res = res[:512*1024]
	}

	return res, nil
}
