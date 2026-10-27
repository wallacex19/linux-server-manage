package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linux-server-manager/backend/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Host:        "127.0.0.1",
		Port:        8080,
		Environment: "testing",
		PublicURL:   "http://localhost:8080",
	}

	// Router sem banco (health handler trata db nil graciosamente)
	router, err := NewRouter(cfg, nil)
	if err != nil {
		t.Fatalf("falha ao criar router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperado status %d, obtido %d", http.StatusOK, w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro ao decodificar resposta JSON: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("esperado status 'healthy', obtido '%s'", resp.Status)
	}

	if resp.Environment != "testing" {
		t.Errorf("esperado environment 'testing', obtido '%s'", resp.Environment)
	}

	if resp.Version != "0.1.0" {
		t.Errorf("esperado versao '0.1.0', obtido '%s'", resp.Version)
	}
}

func TestVersionEndpoint(t *testing.T) {
	cfg := &config.Config{
		Host:        "127.0.0.1",
		Port:        8080,
		Environment: "testing",
		PublicURL:   "http://localhost:8080",
	}
	router, err := NewRouter(cfg, nil)
	if err != nil {
		t.Fatalf("falha ao criar router: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obtido %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}

	if resp["version"] != "0.1.0" {
		t.Errorf("esperado versao '0.1.0', obtido '%s'", resp["version"])
	}
}
