package steamcmd_test

import (
	"path/filepath"
	"testing"

	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/steamcmd"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_steamcmd.db")
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar banco de teste: %v", err)
	}

	return db, func() {
		_ = db.Close()
	}
}

func TestSteamCMD_CheckStatusAndPresets(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := steamcmd.NewTestService(db, "test-node")
	status := svc.CheckStatus()

	if !status.Installed {
		t.Errorf("esperado status.Installed = true no modo dev")
	}

	if len(steamcmd.PopularPresets) == 0 {
		t.Errorf("esperado presets populares de jogos predefinidos")
	}
}

func TestSteamCMD_ServerLifecycleAndSecurity(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	svc := steamcmd.NewTestService(db, "test-node")

	userA := &auth.LinuxUserIdentity{
		Username: "jogador1",
		UID:      1001,
		GID:      1001,
		IsAdmin:  false,
	}

	userB := &auth.LinuxUserIdentity{
		Username: "jogador2",
		UID:      1002,
		GID:      1002,
		IsAdmin:  false,
	}

	admin := &auth.LinuxUserIdentity{
		Username: "admin",
		UID:      1000,
		GID:      1000,
		IsAdmin:  true,
	}

	// 1. Criar servidor pelo jogador 1
	created, err := svc.CreateServer(userA, steamcmd.CreateServerRequest{
		Name:         "Servidor Valheim do Jogador 1",
		ServiceType:  "valheim",
		AppID:        "896660",
		InstallPath:  "/home/jogador1/gameservers/valheim",
		StartCommand: "./valheim_server.x86_64",
		Port:         2456,
		AutoRestart:  false,
	})
	if err != nil {
		t.Fatalf("falha ao criar servidor: %v", err)
	}

	if created.SystemUser != "jogador1" {
		t.Errorf("esperado proprietário 'jogador1', obtido '%s'", created.SystemUser)
	}

	// 2. Listar: Jogador 1 deve ver o servidor
	listA, err := svc.ListServers(userA)
	if err != nil {
		t.Fatalf("falha ao listar para userA: %v", err)
	}
	if len(listA) != 1 {
		t.Errorf("esperado 1 servidor na lista do jogador 1, obtido %d", len(listA))
	}

	// 3. Listar: Jogador 2 NÃO deve ver o servidor do jogador 1 (Isolamento de Contas)
	listB, err := svc.ListServers(userB)
	if err != nil {
		t.Fatalf("falha ao listar para userB: %v", err)
	}
	if len(listB) != 0 {
		t.Errorf("esperado 0 servidores na lista do jogador 2, obtido %d", len(listB))
	}

	// 4. Jogador 2 NÃO pode iniciar nem parar o servidor do jogador 1 (403 Proibido)
	err = svc.StartServer(userB, created.ID)
	if err == nil || err != steamcmd.ErrUnauthorizedServerAction {
		t.Errorf("esperado ErrUnauthorizedServerAction ao jogador 2 tentar iniciar servidor de outro, obtido %v", err)
	}

	// 5. Jogador 1 consegue iniciar seu próprio servidor
	err = svc.StartServer(userA, created.ID)
	if err != nil {
		t.Fatalf("falha ao iniciar servidor pelo próprio dono: %v", err)
	}

	// Verifica se status mudou para running
	listA, _ = svc.ListServers(userA)
	if listA[0].Status != "running" {
		t.Errorf("esperado status 'running', obtido '%s'", listA[0].Status)
	}

	// 6. Administrador tem permissão global de gerenciar e parar
	err = svc.StopServer(admin, created.ID)
	if err != nil {
		t.Fatalf("administrador deve conseguir parar qualquer servidor: %v", err)
	}

	// 7. Excluir servidor
	err = svc.DeleteServer(userA, created.ID)
	if err != nil {
		t.Fatalf("falha ao excluir servidor: %v", err)
	}

	// 8. Testes de segurança: Injeção de Comandos em AppID, InstallPath e Beta
	_, err = svc.CreateServer(userA, steamcmd.CreateServerRequest{
		Name:        "Servidor Malicioso",
		ServiceType: "steamcmd",
		AppID:       "896660; rm -rf /",
	})
	if err == nil {
		t.Errorf("esperado erro ao tentar criar servidor com AppID contendo injeção de comandos")
	}

	_, err = svc.CreateServer(userA, steamcmd.CreateServerRequest{
		Name:        "Servidor Malicioso Path",
		ServiceType: "steamcmd",
		AppID:       "896660",
		InstallPath: "/home/user/games; rm -rf /",
	})
	if err == nil {
		t.Errorf("esperado erro ao tentar criar servidor com InstallPath contendo caracteres especiais")
	}
}
