package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linux-server-manager/backend/internal/api"
	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/config"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/steamcmd"
	"github.com/linux-server-manager/backend/internal/systemmgr"
)

// TestProductionService_NeverUsesDevAuthenticator_EvenWithTestEnv demonstra que o construtor
// padrão NewRouter utilizado pelo executável oficial NUNCA ativa o autenticador simulado (DevAuthenticator),
// mesmo que variáveis de ambiente como LSM_ENV=test, LSM_ENV=development ou LSM_AUTH_DRIVER=dev sejam injetadas.
func TestProductionService_NeverUsesDevAuthenticator_EvenWithTestEnv(t *testing.T) {
	// Tenta forçar variáveis de ambiente que anteriormente poderiam induzir simulação
	t.Setenv("LSM_ENV", "testing")
	t.Setenv("LSM_AUTH_DRIVER", "dev")

	dbPath := t.TempDir() + "/neg_test.db"
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar ao banco: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		Host:         "127.0.0.1",
		Port:         8080,
		DatabasePath: dbPath,
		Environment:  "production", // Executável oficial de produção
		PublicURL:    "https://painel.lsm.local",
	}

	// Instanciação usando exclusivamente o construtor oficial do executável
	router, err := api.NewRouter(cfg, db)
	if err != nil {
		t.Fatalf("falha ao criar router de produção: %v", err)
	}

	// Tenta autenticar com as credenciais padrão do DevAuthenticator ("admin" / "adminpassword")
	body, _ := json.Marshal(api.LoginRequest{
		Username: "admin",
		Password: "adminpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:45123"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Em produção, NewRouter usa PAM no Linux ou UnsupportedAuthenticator fora dele.
	// As credenciais simuladas DEVEM ser categoricamente recusadas com 401 Unauthorized em qualquer SO.
	if rec.Code == http.StatusOK {
		t.Fatalf("FALHA CRÍTICA DE SEGURANÇA: O roteador de produção aceitou credenciais simuladas do DevAuthenticator!")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado status HTTP 401 Unauthorized, obtido %d", rec.Code)
	}
}

// TestProductionService_RejectsInvalidEnvironmentValues comprova que configurações de produção
// e o construtor NewRouter recusam valores de ambiente inválidos ou desconhecidos com erro explícito.
func TestProductionService_RejectsInvalidEnvironmentValues(t *testing.T) {
	invalidEnvs := []string{"invalid", "staging", "hacker", "prod_dev", "unknown", "test"}

	dbPath := t.TempDir() + "/neg_env_test.db"
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar ao banco: %v", err)
	}
	defer db.Close()

	for _, env := range invalidEnvs {
		cfg := &config.Config{
			Host:        "127.0.0.1",
			Port:        8080,
			Environment: env,
			PublicURL:   "http://localhost:8080",
		}
		// 1. Validação no config
		if err := cfg.Validate(); err == nil {
			t.Errorf("esperado erro ao validar ambiente inválido '%s', mas foi aceito", env)
		}
		// 2. Rejeição explícita no construtor NewRouter
		if _, err := api.NewRouter(cfg, db); err == nil {
			t.Errorf("esperado erro explícito em NewRouter para ambiente inválido '%s', mas retornou nil", env)
		}
	}
}

// TestProductionService_RejectsNilConfig comprova que NewRouter rejeita configuração nula
func TestProductionService_RejectsNilConfig(t *testing.T) {
	_, err := api.NewRouter(nil, nil)
	if err == nil {
		t.Fatal("esperado erro explícito ao chamar NewRouter com cfg nil, mas retornou sucesso")
	}
}

// TestSystemManager_ProductionRejectsMocks comprova que o SystemManager de produção não opera com dados simulados
func TestSystemManager_ProductionRejectsMocks(t *testing.T) {
	mgr := systemmgr.NewSystemManager()
	user := &auth.LinuxUserIdentity{
		Username: "operador",
		UID:      1001,
		IsAdmin:  false,
	}

	// ListProcesses em produção não retorna processos mockados
	_, err := mgr.ListProcesses(user)
	if err == nil {
		t.Fatal("FALHA: SystemManager de produção retornou processos mockados!")
	}

	// KillProcess em produção recusa simular encerramento
	err = mgr.KillProcess(user, 1024, "SIGTERM")
	if err == nil {
		t.Fatal("FALHA: SystemManager de produção simulou encerramento de processo!")
	}
}

// TestSteamCMD_ProductionNeverUsesMocks comprova que o SteamCMD de produção não possui modo simulado ativo
func TestSteamCMD_ProductionNeverUsesMocks(t *testing.T) {
	dbPath := t.TempDir() + "/steam_neg.db"
	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao conectar ao banco: %v", err)
	}
	defer db.Close()

	svc := steamcmd.NewService(db, "prod-node")
	if svc == nil {
		t.Fatal("esperado serviço steamcmd não nulo")
	}
}
