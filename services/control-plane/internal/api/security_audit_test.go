package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/linux-server-manager/backend/internal/auth"
	"github.com/linux-server-manager/backend/internal/config"
	"github.com/linux-server-manager/backend/internal/database"
	"github.com/linux-server-manager/backend/internal/steamcmd"
	"github.com/linux-server-manager/backend/internal/systemmgr"
	"github.com/linux-server-manager/backend/internal/terminal"
)

func TestSecurity_CORS_CSWSH_CSRF_And_RemoteAuth(t *testing.T) {
	tempDB, err := os.CreateTemp("", "lsm_security_audit_*.db")
	if err != nil {
		t.Fatalf("falha ao criar temp db: %v", err)
	}
	dbPath := tempDB.Name()
	tempDB.Close()
	defer os.Remove(dbPath)

	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		Host:        "127.0.0.1",
		Port:        8080,
		Environment: "testing",
		PublicURL:   "http://localhost:8080",
	}
	router, err := NewRouterWithOptions(cfg, db, RouterOptions{
		Authenticator: auth.NewDevAuthenticator(),
		SystemManager: systemmgr.NewTestSystemManager(),
		SteamCMD:      steamcmd.NewTestService(db, "local-node"),
	})
	if err != nil {
		t.Fatalf("falha ao criar router: %v", err)
	}

	// =========================================================================
	// 1. CSWSH Check: validação de Origin do WebSocket
	// =========================================================================
	reqLocalWS := httptest.NewRequest("GET", "http://localhost:8080/api/v1/terminal/ws", nil)
	reqLocalWS.Header.Set("Origin", "http://localhost:5173")
	if !terminal.CheckWebSocketOrigin(reqLocalWS) {
		t.Errorf("CSWSH: esperado que localhost:5173 seja autorizado em dev")
	}

	reqAttackerWS := httptest.NewRequest("GET", "http://localhost:8080/api/v1/terminal/ws", nil)
	reqAttackerWS.Header.Set("Origin", "https://evil-attacker.example.com")
	if terminal.CheckWebSocketOrigin(reqAttackerWS) {
		t.Errorf("CSWSH: esperado que evil-attacker.example.com SEJA BLOQUEADO!")
	}

	reqNoOriginWS := httptest.NewRequest("GET", "http://localhost:8080/api/v1/terminal/ws", nil)
	if !terminal.CheckWebSocketOrigin(reqNoOriginWS) {
		t.Errorf("CSWSH: clientes nativos/CLI sem Origin devem ser aceitos")
	}

	// =========================================================================
	// 2. CORS Preflight: Não deve refletir origens arbitrárias com credentials
	// =========================================================================
	reqCorsAttacker := httptest.NewRequest("OPTIONS", "/api/v1/files/list", nil)
	reqCorsAttacker.Header.Set("Origin", "https://malicious-site.com")
	wCorsAttacker := httptest.NewRecorder()
	router.ServeHTTP(wCorsAttacker, reqCorsAttacker)

	if wCorsAttacker.Header().Get("Access-Control-Allow-Origin") == "https://malicious-site.com" {
		t.Errorf("CORS: Falha de segurança! Origem arbitrária foi refletida.")
	}
	if wCorsAttacker.Header().Get("Access-Control-Allow-Credentials") == "true" {
		t.Errorf("CORS: Falha de segurança! Credenciais permitidas para origem não confiável.")
	}

	// =========================================================================
	// 3. CSRF Protection: Rejeição de mutações originadas de domínio externo
	// =========================================================================
	reqCsrfAttack := httptest.NewRequest("POST", "/api/v1/files/create", bytes.NewBufferString(`{"path":"hack.txt"}`))
	reqCsrfAttack.Header.Set("Origin", "https://malicious-site.com")
	reqCsrfAttack.Header.Set("Content-Type", "application/json")
	wCsrfAttack := httptest.NewRecorder()
	router.ServeHTTP(wCsrfAttack, reqCsrfAttack)

	if wCsrfAttack.Code != http.StatusForbidden {
		t.Errorf("CSRF: esperado HTTP 403 Forbidden para requisição mutativa de origem não autorizada, obtido %d", wCsrfAttack.Code)
	}

	// =========================================================================
	// 4. Autenticação para testes de autorização remota (Multi-Server)
	// =========================================================================
	loginBody := bytes.NewBufferString(`{"username":"admin","password":"adminpassword"}`)
	reqLogin := httptest.NewRequest("POST", "/api/v1/auth/login", loginBody)
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	router.ServeHTTP(wLogin, reqLogin)

	cookie := wLogin.Result().Cookies()
	if len(cookie) == 0 {
		t.Fatalf("falha ao autenticar sessão de teste")
	}
	sessionCookie := cookie[0]

	// 4.1 Consulta de nó inexistente deve retornar 404
	reqNonExistent := httptest.NewRequest("GET", "/api/v1/files/list?node_id=uuid-inexistente-1234", nil)
	reqNonExistent.AddCookie(sessionCookie)
	wNonExistent := httptest.NewRecorder()
	router.ServeHTTP(wNonExistent, reqNonExistent)

	if wNonExistent.Code != http.StatusNotFound {
		t.Errorf("esperado 404 para nó inexistente, obtido %d (%s)", wNonExistent.Code, wNonExistent.Body.String())
	}

	// 4.2 Inserção de nó revogado e validação de 403
	_, err = db.Exec(`INSERT INTO nodes (id, name, hostname, ip_address, public_key, status) VALUES ('node-revoked-01', 'Nó Revogado', 'servidor2', '10.0.0.2', 'pub2', 'revoked')`)
	if err != nil {
		t.Fatalf("falha ao inserir nó revogado: %v", err)
	}

	reqRevoked := httptest.NewRequest("GET", "/api/v1/files/list?node_id=node-revoked-01", nil)
	reqRevoked.AddCookie(sessionCookie)
	wRevoked := httptest.NewRecorder()
	router.ServeHTTP(wRevoked, reqRevoked)

	if wRevoked.Code != http.StatusForbidden {
		t.Errorf("esperado 403 Forbidden para nó revogado, obtido %d (%s)", wRevoked.Code, wRevoked.Body.String())
	}

	// 4.3 Inserção de nó offline (sem agente conectado) e validação de 503
	_, err = db.Exec(`INSERT INTO nodes (id, name, hostname, ip_address, public_key, status) VALUES ('node-offline-01', 'Nó Offline', 'servidor3', '10.0.0.3', 'pub3', 'offline')`)
	if err != nil {
		t.Fatalf("falha ao inserir nó offline: %v", err)
	}

	reqOffline := httptest.NewRequest("GET", "/api/v1/files/list?node_id=node-offline-01", nil)
	reqOffline.AddCookie(sessionCookie)
	wOffline := httptest.NewRecorder()
	router.ServeHTTP(wOffline, reqOffline)

	if wOffline.Code != http.StatusServiceUnavailable {
		t.Errorf("esperado 503 Service Unavailable para nó offline, obtido %d (%s)", wOffline.Code, wOffline.Body.String())
	}

	// 4.4 Teste idêntico para SystemManager (Processos em nó inexistente / revogado)
	reqProcRevoked := httptest.NewRequest("GET", "/api/v1/system/processes?node_id=node-revoked-01", nil)
	reqProcRevoked.AddCookie(sessionCookie)
	wProcRevoked := httptest.NewRecorder()
	router.ServeHTTP(wProcRevoked, reqProcRevoked)

	if wProcRevoked.Code != http.StatusForbidden {
		t.Errorf("esperado 403 Forbidden para processos em nó revogado, obtido %d", wProcRevoked.Code)
	}
}

func TestSecurity_ServiceLogsAndActions_StrictAuthorizationAndSanitization(t *testing.T) {
	tempDB, err := os.CreateTemp("", "lsm_servicelogs_audit_*.db")
	if err != nil {
		t.Fatalf("falha ao criar temp db: %v", err)
	}
	dbPath := tempDB.Name()
	tempDB.Close()
	defer os.Remove(dbPath)

	db, err := database.Connect(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		Host:        "127.0.0.1",
		Port:        8080,
		Environment: "testing",
		PublicURL:   "http://localhost:8080",
	}
	router, err := NewRouterWithOptions(cfg, db, RouterOptions{
		Authenticator: auth.NewDevAuthenticator(),
		SystemManager: systemmgr.NewTestSystemManager(),
		SteamCMD:      steamcmd.NewTestService(db, "local-node"),
	})
	if err != nil {
		t.Fatalf("falha ao criar router: %v", err)
	}

	// 1. Não autenticado -> 401 Unauthorized
	reqUnauth := httptest.NewRequest("GET", "/api/v1/system/services/logs?name=caddy.service", nil)
	wUnauth := httptest.NewRecorder()
	router.ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Errorf("esperado 401 Unauthorized para chamada sem autenticação, obtido %d", wUnauth.Code)
	}

	// 2. Login como operador comum (não-admin)
	loginOpBody := bytes.NewBufferString(`{"username":"operador","password":"userpassword"}`)
	reqOpLogin := httptest.NewRequest("POST", "/api/v1/auth/login", loginOpBody)
	reqOpLogin.Header.Set("Content-Type", "application/json")
	wOpLogin := httptest.NewRecorder()
	router.ServeHTTP(wOpLogin, reqOpLogin)

	opCookies := wOpLogin.Result().Cookies()
	if len(opCookies) == 0 {
		t.Fatalf("falha ao autenticar operador comum")
	}
	opCookie := opCookies[0]

	// 2.1 Operador comum tentando visualizar logs -> 403 Forbidden
	reqOpLogs := httptest.NewRequest("GET", "/api/v1/system/services/logs?name=caddy.service", nil)
	reqOpLogs.AddCookie(opCookie)
	wOpLogs := httptest.NewRecorder()
	router.ServeHTTP(wOpLogs, reqOpLogs)
	if wOpLogs.Code != http.StatusForbidden {
		t.Errorf("esperado 403 Forbidden para operador comum consultando logs de serviço, obtido %d", wOpLogs.Code)
	}

	// 2.2 Operador comum tentando executar ação de serviço -> 403 Forbidden
	reqOpAction := httptest.NewRequest("POST", "/api/v1/system/services/action", bytes.NewBufferString(`{"name":"caddy.service","action":"restart"}`))
	reqOpAction.AddCookie(opCookie)
	reqOpAction.Header.Set("Content-Type", "application/json")
	wOpAction := httptest.NewRecorder()
	router.ServeHTTP(wOpAction, reqOpAction)
	if wOpAction.Code != http.StatusForbidden {
		t.Errorf("esperado 403 Forbidden para operador comum executando ação em serviço, obtido %d", wOpAction.Code)
	}

	// 3. Login como admin
	loginAdminBody := bytes.NewBufferString(`{"username":"admin","password":"adminpassword"}`)
	reqAdminLogin := httptest.NewRequest("POST", "/api/v1/auth/login", loginAdminBody)
	reqAdminLogin.Header.Set("Content-Type", "application/json")
	wAdminLogin := httptest.NewRecorder()
	router.ServeHTTP(wAdminLogin, reqAdminLogin)

	adminCookies := wAdminLogin.Result().Cookies()
	if len(adminCookies) == 0 {
		t.Fatalf("falha ao autenticar admin")
	}
	adminCookie := adminCookies[0]

	// 3.1 Admin consultando logs válidos -> 200 OK
	reqAdminLogs := httptest.NewRequest("GET", "/api/v1/system/services/logs?name=caddy.service&lines=50", nil)
	reqAdminLogs.AddCookie(adminCookie)
	wAdminLogs := httptest.NewRecorder()
	router.ServeHTTP(wAdminLogs, reqAdminLogs)
	if wAdminLogs.Code != http.StatusOK {
		t.Errorf("esperado 200 OK para admin consultando logs, obtido %d (%s)", wAdminLogs.Code, wAdminLogs.Body.String())
	}

	// 3.2 Tentativa de Command Injection no nome do serviço -> 400 Bad Request
	injectionNames := []string{
		"caddy; rm -rf /",
		"caddy && cat /etc/shadow",
		"caddy`id`",
		"caddy$(whoami)",
		"../caddy.service",
		"invalid service name with spaces",
	}
	for _, badName := range injectionNames {
		reqBad := httptest.NewRequest("GET", "/api/v1/system/services/logs?name="+url.QueryEscape(badName), nil)
		reqBad.AddCookie(adminCookie)
		wBad := httptest.NewRecorder()
		router.ServeHTTP(wBad, reqBad)
		if wBad.Code != http.StatusBadRequest {
			t.Errorf("esperado 400 Bad Request para nome malicioso '%s', obtido %d", badName, wBad.Code)
		}
	}

	// 3.3 Parâmetro lines fora dos limites (1..500) -> 400 Bad Request
	badLines := []string{"0", "-10", "501", "99999", "abc"}
	for _, bl := range badLines {
		reqBadLines := httptest.NewRequest("GET", "/api/v1/system/services/logs?name=caddy.service&lines="+url.QueryEscape(bl), nil)
		reqBadLines.AddCookie(adminCookie)
		wBadLines := httptest.NewRecorder()
		router.ServeHTTP(wBadLines, reqBadLines)
		if wBadLines.Code != http.StatusBadRequest {
			t.Errorf("esperado 400 Bad Request para lines='%s', obtido %d", bl, wBadLines.Code)
		}
	}
}
