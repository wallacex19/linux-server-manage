package systemmgr

import (
	"testing"

	"github.com/linux-server-manager/backend/internal/auth"
)

func TestSystemManagerPermissionsAndLifecycle(t *testing.T) {
	mgr := NewTestSystemManager() // Adaptador simulado explicitamente injetado para testes unitários

	normalUser := &auth.LinuxUserIdentity{
		Username: "operador",
		UID:      1001,
		IsAdmin:  false,
	}

	adminUser := &auth.LinuxUserIdentity{
		Username: "admin",
		UID:      1000,
		IsAdmin:  true,
	}

	// 1. Listagem de processos
	procs, err := mgr.ListProcesses(normalUser)
	if err != nil {
		t.Fatalf("falha ao listar processos: %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("esperado ao menos 1 processo listado")
	}

	// 2. Operador não pode encerrar processo root (PID 1)
	err = mgr.KillProcess(normalUser, 1, "SIGTERM")
	if err != ErrUnauthorizedProcessAction {
		t.Fatalf("esperado ErrUnauthorizedProcessAction para operador matando PID 1, obtido: %v", err)
	}

	// 3. Operador pode encerrar seu próprio processo (PID 2048 tem UID 1001)
	err = mgr.KillProcess(normalUser, 2048, "SIGTERM")
	if err != nil {
		t.Fatalf("falha ao operador encerrar seu próprio processo: %v", err)
	}

	// 4. Operador não pode parar serviços systemd
	err = mgr.ControlService(normalUser, "caddy.service", "stop")
	if err != ErrUnauthorizedServiceAction {
		t.Fatalf("esperado ErrUnauthorizedServiceAction para operador gerenciando serviço, obtido: %v", err)
	}

	// 5. Administrador pode parar o serviço
	err = mgr.ControlService(adminUser, "caddy.service", "stop")
	if err != nil {
		t.Fatalf("falha ao admin parar serviço: %v", err)
	}

	// 6. Consulta de logs do serviço: operador comum deve ser bloqueado
	_, err = mgr.GetServiceLogs(normalUser, "caddy.service", 50)
	if err != ErrUnauthorizedServiceAction {
		t.Fatalf("esperado ErrUnauthorizedServiceAction para operador comum visualizando logs, obtido: %v", err)
	}

	// 7. Administrador pode obter logs do serviço
	logs, err := mgr.GetServiceLogs(adminUser, "caddy.service", 50)
	if err != nil {
		t.Fatalf("falha ao admin obter logs: %v", err)
	}
	if logs == "" {
		t.Fatal("logs não devem ser vazios")
	}
}
