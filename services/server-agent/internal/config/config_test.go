package config_test

import (
	"strings"
	"testing"

	"github.com/linux-server-manager/agent/internal/config"
)

func TestConfig_Environment_Empty_DefaultsToProduction(t *testing.T) {
	cfg := &config.Config{
		Environment: "",
		PanelURL:    "http://127.0.0.1:8080",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("esperado erro: ambiente ausente deve assumir produção e rejeitar http://")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("esperado menção a HTTPS no erro, obtido: %v", err)
	}
}

func TestConfig_Environment_Invalid_Rejected(t *testing.T) {
	invalidEnvs := []string{"staging", "prod", "homolog", "invalid"}
	for _, env := range invalidEnvs {
		cfg := &config.Config{
			Environment: env,
			PanelURL:    "https://painel.exemplo.com",
		}
		err := cfg.Validate()
		if err == nil {
			t.Errorf("esperado erro para ambiente inválido '%s', obtido nil", env)
		}
	}
}

func TestConfig_Environment_Production_RejectsHTTP(t *testing.T) {
	cfg := &config.Config{
		Environment: "production",
		PanelURL:    "http://painel.exemplo.com",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("esperado erro ao usar http:// em produção")
	}

	cfgValid := &config.Config{
		Environment: "production",
		PanelURL:    "https://painel.exemplo.com",
	}
	if err := cfgValid.Validate(); err != nil {
		t.Fatalf("esperado sucesso para HTTPS em produção, obtido: %v", err)
	}
}

func TestConfig_Environment_Development_LoopbackAllowed(t *testing.T) {
	validDevURLs := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
		"https://painel.exemplo.com",
	}
	for _, u := range validDevURLs {
		cfg := &config.Config{
			Environment: "development",
			PanelURL:    u,
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("esperado sucesso para URL '%s' em desenvolvimento, obtido: %v", u, err)
		}
	}
}

func TestConfig_Environment_Development_RemoteInsecureRejected(t *testing.T) {
	cfg := &config.Config{
		Environment: "development",
		PanelURL:    "http://servidor-remoto.exemplo.com:8080",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("esperado erro para URL remota insegura (http://) em desenvolvimento")
	}
}

func TestConfig_EmbeddedCredentials_Rejected(t *testing.T) {
	cfg := &config.Config{
		Environment: "production",
		PanelURL:    "https://admin:secret123@painel.exemplo.com",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("esperado erro para URL com credenciais embutidas")
	}
	if !strings.Contains(err.Error(), "credenciais embutidas") {
		t.Errorf("esperado erro sobre credenciais embutidas, obtido: %v", err)
	}
}
