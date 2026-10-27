package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linux-server-manager/backend/internal/monitoring"
)

func TestMetricsEndpoints(t *testing.T) {
	router := setupTestRouter()

	// 1. Sem autenticação deve retornar 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/current", nil)
	unauthW := httptest.NewRecorder()
	router.ServeHTTP(unauthW, unauthReq)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("esperado status 401 para rota não autenticada, obtido %d", unauthW.Code)
	}

	// 2. Realiza login para obter cookie de sessão
	loginPayload := LoginRequest{
		Username: "admin",
		Password: "adminpassword",
	}
	body, _ := json.Marshal(loginPayload)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)

	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "lsm_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("cookie de sessão não retornado")
	}

	// 3. Consulta /api/v1/metrics/current com autenticação
	currentReq := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/current", nil)
	currentReq.AddCookie(sessionCookie)
	currentW := httptest.NewRecorder()
	router.ServeHTTP(currentW, currentReq)

	if currentW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /metrics/current, obtido %d", currentW.Code)
	}

	var snap monitoring.ServerMetricsSnapshot
	if err := json.Unmarshal(currentW.Body.Bytes(), &snap); err != nil {
		t.Fatalf("erro ao decodificar JSON de métricas atuais: %v", err)
	}

	if snap.CPU.Cores <= 0 {
		t.Errorf("esperado contagem de cores > 0, obtido %d", snap.CPU.Cores)
	}
	if snap.Memory.TotalBytes <= 0 {
		t.Errorf("esperado TotalBytes > 0, obtido %d", snap.Memory.TotalBytes)
	}

	// 4. Consulta /api/v1/metrics/history
	histReq := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/history", nil)
	histReq.AddCookie(sessionCookie)
	histW := httptest.NewRecorder()
	router.ServeHTTP(histW, histReq)

	if histW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /metrics/history, obtido %d", histW.Code)
	}

	var history []*monitoring.ServerMetricsSnapshot
	if err := json.Unmarshal(histW.Body.Bytes(), &history); err != nil {
		t.Fatalf("erro ao decodificar array de histórico de métricas: %v", err)
	}

	if len(history) == 0 {
		t.Error("esperado ao menos 1 snapshot no histórico")
	}
}
