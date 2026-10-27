package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/config"
)

func setupTestRouter() http.Handler {
	cfg := &config.Config{
		Host:        "127.0.0.1",
		Port:        8080,
		Environment: "testing",
		PublicURL:   "http://localhost:8080",
	}
	r, err := NewRouterWithOptions(cfg, nil, RouterOptions{
		Authenticator: auth.NewDevAuthenticator(),
	})
	if err != nil {
		panic(err)
	}
	return r
}

func TestAuthLoginSuccessAndMe(t *testing.T) {
	router := setupTestRouter()

	// 1. Login com credenciais válidas do dev provider
	loginPayload := LoginRequest{
		Username: "admin",
		Password: "adminpassword",
	}
	body, _ := json.Marshal(loginPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperado status 200, obtido %d: %s", w.Code, w.Body.String())
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("erro ao deserializar LoginResponse: %v", err)
	}

	if loginResp.Token == "" {
		t.Fatal("token não deve ser vazio")
	}

	if loginResp.User.Username != "admin" {
		t.Errorf("esperado usuário 'admin', obtido '%s'", loginResp.User.Username)
	}

	if !loginResp.User.IsAdmin {
		t.Error("esperado IsAdmin=true")
	}

	// 2. Extrai cookie da resposta
	cookie := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookie {
		if c.Name == "lsm_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("cookie 'lsm_session' não foi setado")
	}

	// 3. Testa /api/v1/auth/me usando o cookie
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meW := httptest.NewRecorder()

	router.ServeHTTP(meW, meReq)

	if meW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /auth/me, obtido %d", meW.Code)
	}

	var meUser auth.LinuxUserIdentity
	if err := json.Unmarshal(meW.Body.Bytes(), &meUser); err != nil {
		t.Fatalf("erro ao deserializar /auth/me: %v", err)
	}

	if meUser.Username != "admin" {
		t.Errorf("esperado username 'admin', obtido '%s'", meUser.Username)
	}

	// 4. Testa /api/v1/system/info autenticado
	sysReq := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	sysReq.AddCookie(sessionCookie)
	sysW := httptest.NewRecorder()

	router.ServeHTTP(sysW, sysReq)
	if sysW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /system/info, obtido %d", sysW.Code)
	}

	// 5. Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()

	router.ServeHTTP(logoutW, logoutReq)
	if logoutW.Code != http.StatusOK {
		t.Fatalf("esperado status 200 em /auth/logout, obtido %d", logoutW.Code)
	}

	// 6. /api/v1/auth/me deve falhar após logout
	meAfterReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meAfterReq.AddCookie(sessionCookie)
	meAfterW := httptest.NewRecorder()

	router.ServeHTTP(meAfterW, meAfterReq)
	if meAfterW.Code != http.StatusUnauthorized {
		t.Fatalf("esperado status 401 após logout, obtido %d", meAfterW.Code)
	}
}

func TestAuthLoginInvalidCredentials(t *testing.T) {
	router := setupTestRouter()

	loginPayload := LoginRequest{
		Username: "admin",
		Password: "senha_errada",
	}
	body, _ := json.Marshal(loginPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperado status 401, obtido %d", w.Code)
	}
}

func TestProtectedEndpointWithoutAuth(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperado status 401 para rota protegida sem credenciais, obtido %d", w.Code)
	}
}
